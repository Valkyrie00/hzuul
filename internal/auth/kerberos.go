package auth

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// Kerberos implements SPNEGO authentication by delegating the Negotiate
// handshake to the system's curl binary (which uses native GSSAPI / GSS.framework).
// This matches the behavior of Python's requests_kerberos exactly — it reads
// the TGT acquired via kinit from the system's credential cache.
type Kerberos struct {
	jar         *cookiejar.Jar
	verifySSL   bool
	caCert      string
	accessToken string
}

// NewKerberos bootstraps a Kerberos session by running curl --negotiate against
// the given URL. The session cookies are captured and reused by the Go HTTP client.
// It also attempts to obtain an OIDC Bearer token for admin write operations.
func NewKerberos(targetURL string, verifySSL bool, caCert string) (*Kerberos, error) {
	if _, err := exec.LookPath("curl"); err != nil {
		return nil, fmt.Errorf("curl not found in PATH — required for Kerberos auth")
	}

	jar, _ := cookiejar.New(nil)
	k := &Kerberos{jar: jar, verifySSL: verifySSL, caCert: caCert}

	if err := k.negotiate(targetURL); err != nil {
		return nil, err
	}

	if token, err := k.acquireOIDCToken(targetURL); err != nil {
		slog.Debug("kerberos: could not acquire OIDC token (admin ops may fail)", "error", err)
	} else {
		k.accessToken = token
		slog.Debug("kerberos: acquired OIDC bearer token for admin operations")
	}

	return k, nil
}

func (k *Kerberos) BearerToken() string { return k.accessToken }

func (k *Kerberos) HTTPClient(_ *http.Transport) HTTPDoer {
	tlsCfg := &tls.Config{
		InsecureSkipVerify: !k.verifySSL,
	}
	if k.caCert != "" {
		pool, err := loadCACertPool(k.caCert)
		if err == nil {
			tlsCfg.RootCAs = pool
		}
	}
	return &http.Client{
		Jar:       k.jar,
		Timeout:   30 * time.Second,
		Transport: &http.Transport{TLSClientConfig: tlsCfg},
	}
}

func (k *Kerberos) Validate() error {
	return nil
}

// negotiate runs curl --negotiate to perform SPNEGO authentication and captures
// the resulting session cookies into the jar.
func (k *Kerberos) negotiate(targetURL string) error {
	if slog.Default().Enabled(context.TODO(), slog.LevelDebug) {
		if out, err := exec.Command("klist").CombinedOutput(); err == nil {
			slog.Debug("kerberos ticket cache", "klist", string(out))
		} else {
			slog.Debug("klist not available", "error", err)
		}
	}

	cookieFile, err := os.CreateTemp("", "hzuul-cookies-*")
	if err != nil {
		return fmt.Errorf("creating temp cookie file: %w", err)
	}
	cookiePath := cookieFile.Name()
	cookieFile.Close()
	defer os.Remove(cookiePath)

	args := []string{
		"--negotiate", "-u", ":",
		"--location-trusted", "-s",
		"-c", cookiePath,
		"-o", "/dev/null",
		"-w", "%{http_code}",
	}
	if !k.verifySSL {
		args = append(args, "-k")
	}
	if k.caCert != "" {
		args = append(args, "--cacert", k.caCert)
	}
	if slog.Default().Enabled(context.TODO(), slog.LevelDebug) {
		// Replace -s with -v for verbose curl output when debugging
		for i, a := range args {
			if a == "-s" {
				args[i] = "-v"
				break
			}
		}
	}
	args = append(args, targetURL)

	slog.Debug("running curl negotiate", "args", strings.Join(args, " "))
	cmd := exec.Command("curl", args...)
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("curl --negotiate failed (exit %d): %s — have you run 'kinit'?",
				exitErr.ExitCode(), stderrBuf.String())
		}
		return fmt.Errorf("curl --negotiate failed: %w", err)
	}

	httpStatus := strings.TrimSpace(string(out))
	slog.Debug("curl completed", "http_status", httpStatus)

	if slog.Default().Enabled(context.TODO(), slog.LevelDebug) {
		for _, line := range strings.Split(stderrBuf.String(), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "> ") || strings.HasPrefix(line, "< ") ||
				strings.Contains(line, "Auth") || strings.Contains(line, "Negotiate") ||
				strings.Contains(line, "gss") || strings.Contains(line, "GSS") ||
				strings.Contains(line, "Location:") {
				slog.Debug("curl", "line", line)
			}
		}
	}

	cookieData, err := os.ReadFile(cookiePath)
	if err != nil {
		return fmt.Errorf("reading cookie file: %w", err)
	}

	cookies, err := parseNetscapeCookies(string(cookieData))
	if err != nil {
		return err
	}

	u, err := url.Parse(targetURL)
	if err != nil {
		return err
	}

	slog.Debug("parsed cookies", "count", len(cookies))

	if len(cookies) == 0 {
		return fmt.Errorf("no session cookies obtained (HTTP %s) — SPNEGO may have failed (run 'kinit' and retry)",
			httpStatus)
	}

	// Group cookies by domain so SetCookies accepts each group
	// (the jar rejects cookies whose Domain doesn't match the URL host).
	byDomain := make(map[string][]*http.Cookie)
	for _, c := range cookies {
		byDomain[c.Domain] = append(byDomain[c.Domain], c)
	}
	for domain, dc := range byDomain {
		du := &url.URL{Scheme: "https", Host: domain}
		k.jar.SetCookies(du, dc)
		slog.Debug("stored cookies", "domain", domain, "count", len(k.jar.Cookies(du)))
	}

	stored := k.jar.Cookies(u)
	if len(stored) == 0 {
		return fmt.Errorf("SPNEGO completed (HTTP %s) but no session cookies for %s — the server may require OIDC auth instead",
			httpStatus, u.Host)
	}

	slog.Debug("kerberos auth complete", "target", u.Host, "cookies", len(stored))
	return nil
}

// acquireOIDCToken performs an OIDC authorization code flow with PKCE,
// using Kerberos/SPNEGO for authentication at the IdP (Keycloak).
// This obtains a Bearer token needed for Zuul admin write operations.
func (k *Kerberos) acquireOIDCToken(targetURL string) (string, error) {
	// Step 1: discover the OIDC auth URL by following the initial 302
	oidcAuthURL, err := k.discoverOIDCRedirect(targetURL)
	if err != nil {
		return "", fmt.Errorf("discovering OIDC endpoint: %w", err)
	}
	slog.Debug("kerberos oidc: discovered auth URL", "url", oidcAuthURL)

	parsed, err := url.Parse(oidcAuthURL)
	if err != nil {
		return "", fmt.Errorf("parsing OIDC auth URL: %w", err)
	}

	clientID := parsed.Query().Get("client_id")
	redirectURI := parsed.Query().Get("redirect_uri")
	if clientID == "" || redirectURI == "" {
		return "", fmt.Errorf("missing client_id or redirect_uri in OIDC URL")
	}

	// Step 2: discover the token endpoint from well-known config
	issuer := fmt.Sprintf("%s://%s%s", parsed.Scheme, parsed.Host,
		strings.TrimSuffix(parsed.Path, "/protocol/openid-connect/auth"))
	tokenEndpoint, err := k.discoverTokenEndpoint(issuer)
	if err != nil {
		return "", fmt.Errorf("discovering token endpoint: %w", err)
	}
	slog.Debug("kerberos oidc: token endpoint", "url", tokenEndpoint)

	// Step 3: generate PKCE code verifier/challenge
	verifier := generateCodeVerifier()
	challenge := generateCodeChallenge(verifier)

	// Step 4: build our OIDC auth request with PKCE
	state := generateCodeVerifier()[:16]
	nonce := generateCodeVerifier()[:16]
	authParams := url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"scope":                 {"openid profile roles"},
		"state":                 {state},
		"nonce":                 {nonce},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	authEndpoint := fmt.Sprintf("%s://%s%s?%s",
		parsed.Scheme, parsed.Host, parsed.Path, authParams.Encode())

	// Step 5: use curl --negotiate to authenticate and capture the auth code
	code, err := k.negotiateOIDCCode(authEndpoint, redirectURI)
	if err != nil {
		return "", fmt.Errorf("SPNEGO OIDC flow: %w", err)
	}
	slog.Debug("kerberos oidc: captured auth code", "code_len", len(code))

	// Step 6: exchange auth code for tokens at the token endpoint
	token, err := k.exchangeCodeForToken(tokenEndpoint, code, redirectURI, clientID, verifier)
	if err != nil {
		return "", fmt.Errorf("token exchange: %w", err)
	}

	return token, nil
}

func (k *Kerberos) discoverOIDCRedirect(targetURL string) (string, error) {
	args := []string{
		"-s", "-o", "/dev/null",
		"-w", "%{redirect_url}",
		"--max-redirs", "0",
	}
	if !k.verifySSL {
		args = append(args, "-k")
	}
	if k.caCert != "" {
		args = append(args, "--cacert", k.caCert)
	}
	args = append(args, targetURL)

	out, err := exec.Command("curl", args...).Output()
	if err != nil {
		// curl returns exit code 47 for --max-redirs exceeded, which is fine
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 47 {
			return "", fmt.Errorf("curl discover: exit %d", exitErr.ExitCode())
		}
	}
	redirectURL := strings.TrimSpace(string(out))
	if redirectURL == "" || !strings.Contains(redirectURL, "openid-connect") {
		return "", fmt.Errorf("no OIDC redirect found (got: %q)", redirectURL)
	}
	return redirectURL, nil
}

func (k *Kerberos) discoverTokenEndpoint(issuer string) (string, error) {
	wellKnown := issuer + "/.well-known/openid-configuration"

	args := []string{"-s"}
	if !k.verifySSL {
		args = append(args, "-k")
	}
	if k.caCert != "" {
		args = append(args, "--cacert", k.caCert)
	}
	args = append(args, wellKnown)

	out, err := exec.Command("curl", args...).Output()
	if err != nil {
		return "", fmt.Errorf("fetching %s: %w", wellKnown, err)
	}

	var cfg struct {
		TokenEndpoint string `json:"token_endpoint"`
	}
	if err := json.Unmarshal(out, &cfg); err != nil {
		return "", fmt.Errorf("parsing OIDC config: %w", err)
	}
	if cfg.TokenEndpoint == "" {
		return "", fmt.Errorf("no token_endpoint in OIDC config")
	}
	return cfg.TokenEndpoint, nil
}

func (k *Kerberos) negotiateOIDCCode(authEndpoint, redirectURI string) (string, error) {
	// Use -w to capture the final redirect URL without following it to
	// Zuul's redirect_uri (where mod_auth_openidc would consume the code).
	// --location-trusted follows redirects within the IdP, -D - dumps all headers.
	args := []string{
		"--negotiate", "-u", ":",
		"--location-trusted", "-s",
		"-D", "-",
		"-o", "/dev/null",
	}
	if !k.verifySSL {
		args = append(args, "-k")
	}
	if k.caCert != "" {
		args = append(args, "--cacert", k.caCert)
	}
	args = append(args, authEndpoint)

	out, _ := exec.Command("curl", args...).CombinedOutput()
	output := string(out)

	// Extract code from Location headers that redirect to the redirect_uri
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		lower := strings.ToLower(line)
		if !strings.HasPrefix(lower, "location:") {
			continue
		}
		locURL := strings.TrimSpace(line[len("location:"):])
		if u, err := url.Parse(locURL); err == nil {
			if code := u.Query().Get("code"); code != "" {
				return code, nil
			}
		}
	}

	// Also check for code in response body (form_post mode)
	codeMatch := regexp.MustCompile(`name="code"\s+value="([^"]+)"`).FindStringSubmatch(output)
	if len(codeMatch) >= 2 {
		return codeMatch[1], nil
	}
	codeURLMatch := regexp.MustCompile(`[?&]code=([^&"'\s]+)`).FindStringSubmatch(output)
	if len(codeURLMatch) >= 2 {
		return codeURLMatch[1], nil
	}

	return "", fmt.Errorf("could not extract auth code from OIDC response")
}

func (k *Kerberos) exchangeCodeForToken(tokenEndpoint, code, redirectURI, clientID, codeVerifier string) (string, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {clientID},
		"code_verifier": {codeVerifier},
	}

	args := []string{
		"-s", "-X", "POST",
		"-H", "Content-Type: application/x-www-form-urlencoded",
		"-d", data.Encode(),
	}
	if !k.verifySSL {
		args = append(args, "-k")
	}
	if k.caCert != "" {
		args = append(args, "--cacert", k.caCert)
	}
	args = append(args, tokenEndpoint)

	out, err := exec.Command("curl", args...).Output()
	if err != nil {
		return "", fmt.Errorf("token exchange request: %w", err)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		IDToken     string `json:"id_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.Unmarshal(out, &tokenResp); err != nil {
		return "", fmt.Errorf("parsing token response: %w (body: %s)", err, string(out)[:min(200, len(out))])
	}
	if tokenResp.Error != "" {
		return "", fmt.Errorf("%s: %s", tokenResp.Error, tokenResp.ErrorDesc)
	}

	if tokenResp.AccessToken != "" {
		return tokenResp.AccessToken, nil
	}
	if tokenResp.IDToken != "" {
		return tokenResp.IDToken, nil
	}
	return "", fmt.Errorf("no token in response")
}

func generateCodeVerifier() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func generateCodeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func loadCACertPool(path string) (*x509.CertPool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading CA cert %s: %w", path, err)
	}
	pool, err := x509.SystemCertPool()
	if err != nil {
		pool = x509.NewCertPool()
	}
	pool.AppendCertsFromPEM(data)
	return pool, nil
}

// parseNetscapeCookies parses curl's -c (cookie-jar) output format:
//
//	# Netscape HTTP Cookie File
//	.domain.com	TRUE	/path	FALSE	0	name	value
func parseNetscapeCookies(data string) ([]*http.Cookie, error) {
	var cookies []*http.Cookie
	scanner := bufio.NewScanner(strings.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimPrefix(line, "#HttpOnly_")
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 7 {
			continue
		}
		cookies = append(cookies, &http.Cookie{
			Name:   fields[5],
			Value:  fields[6],
			Domain: fields[0],
			Path:   fields[2],
			Secure: strings.EqualFold(fields[3], "TRUE"),
		})
	}
	return cookies, scanner.Err()
}
