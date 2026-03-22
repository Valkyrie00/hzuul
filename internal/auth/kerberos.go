package auth

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Kerberos implements SPNEGO authentication by delegating the Negotiate
// handshake to the system's curl binary (which uses native GSSAPI / GSS.framework).
// This matches the behavior of Python's requests_kerberos exactly — it reads
// the TGT acquired via kinit from the system's credential cache.
type Kerberos struct {
	jar       *cookiejar.Jar
	verifySSL bool
	caCert    string
}

// NewKerberos bootstraps a Kerberos session by running curl --negotiate against
// the given URL. The session cookies are captured and reused by the Go HTTP client.
func NewKerberos(targetURL string, verifySSL bool, caCert string) (*Kerberos, error) {
	if _, err := exec.LookPath("curl"); err != nil {
		return nil, fmt.Errorf("curl not found in PATH — required for Kerberos auth")
	}

	jar, _ := cookiejar.New(nil)
	k := &Kerberos{jar: jar, verifySSL: verifySSL, caCert: caCert}

	if err := k.negotiate(targetURL); err != nil {
		return nil, err
	}

	return k, nil
}

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
