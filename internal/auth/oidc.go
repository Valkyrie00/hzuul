package auth

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// OIDC implements authentication via OpenID Connect with Keycloak.
type OIDC struct {
	client      *http.Client
	baseURL     string
	username    string
	password    string
	verifySSL   bool
	accessToken string
}

func NewOIDC(baseURL, username, password string, verifySSL bool, caCert string) (*OIDC, error) {
	jar, _ := cookiejar.New(nil)
	tlsCfg := &tls.Config{
		InsecureSkipVerify: !verifySSL,
	}
	if caCert != "" {
		pool, err := loadCACertPool(caCert)
		if err == nil {
			tlsCfg.RootCAs = pool
		}
	}
	baseTransport := &http.Transport{
		TLSClientConfig: tlsCfg,
	}
	client := &http.Client{
		Jar:       jar,
		Timeout:   60 * time.Second,
		Transport: &browserTransport{base: baseTransport},
	}

	o := &OIDC{
		client:    client,
		baseURL:   baseURL,
		username:  username,
		password:  password,
		verifySSL: verifySSL,
	}

	if err := o.login(); err != nil {
		return nil, fmt.Errorf("OIDC login: %w", err)
	}

	if token, err := o.acquireToken(); err != nil {
		slog.Debug("oidc: could not acquire bearer token (admin ops may fail)", "error", err)
	} else {
		o.accessToken = token
		slog.Debug("oidc: acquired bearer token for admin operations")
	}

	return o, nil
}

func (o *OIDC) HTTPClient(_ *http.Transport) HTTPDoer {
	return o.client
}

func (o *OIDC) BearerToken() string { return o.accessToken }

func (o *OIDC) Validate() error {
	return nil
}

func (o *OIDC) acquireToken() (string, error) {
	// Discover the OIDC auth URL from the initial 302 redirect
	resp, err := http.Get(o.baseURL + "/api/tenants")
	if err != nil {
		// Follow redirects manually to find the OIDC endpoint
	}
	if resp != nil {
		resp.Body.Close()
	}

	// Use our already-established session to discover the OIDC config
	discoverResp, body, err := o.doGet(o.baseURL + "/api/tenants")
	if err != nil {
		return "", err
	}
	_ = body

	// Find the Keycloak issuer from the initial redirect chain
	var issuer, clientID string
	for _, rURL := range discoverResp.Request.Response.Header.Values("Location") {
		if strings.Contains(rURL, "openid-connect") {
			if u, err := url.Parse(rURL); err == nil {
				clientID = u.Query().Get("client_id")
				issuer = fmt.Sprintf("%s://%s%s", u.Scheme, u.Host,
					strings.TrimSuffix(u.Path, "/protocol/openid-connect/auth"))
			}
			break
		}
	}

	// If we couldn't get it from redirect, try the non-authenticated flow
	if issuer == "" {
		tlsCfg := &tls.Config{InsecureSkipVerify: !o.verifySSL}
		noFollowClient := &http.Client{
			Transport: &http.Transport{TLSClientConfig: tlsCfg},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		rr, err := noFollowClient.Get(o.baseURL + "/api/tenants")
		if err != nil {
			return "", fmt.Errorf("discovering OIDC: %w", err)
		}
		rr.Body.Close()
		loc := rr.Header.Get("Location")
		if u, err := url.Parse(loc); err == nil && strings.Contains(loc, "openid-connect") {
			clientID = u.Query().Get("client_id")
			issuer = fmt.Sprintf("%s://%s%s", u.Scheme, u.Host,
				strings.TrimSuffix(u.Path, "/protocol/openid-connect/auth"))
		}
	}

	if issuer == "" || clientID == "" {
		return "", fmt.Errorf("could not discover OIDC issuer/client_id")
	}

	// Fetch token endpoint from well-known config
	wellKnown := issuer + "/.well-known/openid-configuration"
	wr, wbody, err := o.doGet(wellKnown)
	if err != nil {
		return "", fmt.Errorf("fetching OIDC config: %w", err)
	}
	_ = wr

	type oidcConfig struct {
		TokenEndpoint string `json:"token_endpoint"`
	}
	var cfg oidcConfig
	if err := json.Unmarshal([]byte(wbody), &cfg); err != nil {
		return "", fmt.Errorf("parsing OIDC config: %w", err)
	}
	if cfg.TokenEndpoint == "" {
		return "", fmt.Errorf("no token_endpoint in OIDC config")
	}

	// Try ROPC grant
	tokenData := url.Values{
		"grant_type": {"password"},
		"client_id":  {clientID},
		"username":   {o.username},
		"password":   {o.password},
		"scope":      {"openid profile roles"},
	}

	tokenResp, tokenBody, err := o.doPost(cfg.TokenEndpoint, tokenData)
	if err != nil {
		return "", fmt.Errorf("token request: %w", err)
	}
	if tokenResp.StatusCode >= 400 {
		return "", fmt.Errorf("token request: %s", tokenBody[:min(200, len(tokenBody))])
	}

	var tr struct {
		AccessToken string `json:"access_token"`
		IDToken     string `json:"id_token"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal([]byte(tokenBody), &tr); err != nil {
		return "", fmt.Errorf("parsing token: %w", err)
	}
	if tr.Error != "" {
		return "", fmt.Errorf("token error: %s", tr.Error)
	}
	if tr.AccessToken != "" {
		return tr.AccessToken, nil
	}
	if tr.IDToken != "" {
		return tr.IDToken, nil
	}
	return "", fmt.Errorf("no token in response")
}

// login performs the full OIDC authentication flow by following every HTML form
// it encounters: Kerberos-fallback auto-submit forms, the Keycloak login form,
// and the OIDC response_mode=form_post callback form with hidden code/state fields.
//
// After successful credential submission, Keycloak may return a JavaScript page
// (history.replaceState) instead of an HTTP redirect. When this happens we GET
// the URL from replaceState to continue the *same* Keycloak auth session — this
// may return a required-action form or redirect back to Zuul with the auth code.
func (o *OIDC) login() error {
	zuulURL, _ := url.Parse(o.baseURL)

	slog.Debug("oidc: starting login", "url", o.baseURL+"/api/tenants")
	resp, body, err := o.doGet(o.baseURL + "/api/tenants")
	if err != nil {
		return fmt.Errorf("initial request: %w", err)
	}
	slog.Debug("oidc: initial response", "status", resp.StatusCode, "url", resp.Request.URL.String(), "hasForm", containsForm(body))

	for attempt := range 20 {
		atZuul := resp.Request.URL.Hostname() == zuulURL.Hostname()

		if resp.StatusCode == 200 && !containsForm(body) && atZuul {
			slog.Debug("oidc: login successful", "attempt", attempt, "cookies", len(o.client.Jar.Cookies(zuulURL)))
			return nil
		}

		// On the IdP with 200 + no form → JavaScript post-login page.
		// Keycloak uses either history.replaceState or a timed JS redirect
		// (via an <a id="finishLoginLink"> element). Extract whichever URL
		// is present and GET it to advance the auth flow.
		if resp.StatusCode == 200 && !containsForm(body) && !atZuul {
			nextURL := extractReplaceStateURL(body)
			source := "replaceState"
			if nextURL == "" {
				nextURL = extractLinkHref(body)
				source = "finishLink"
			}
			if nextURL == "" {
				return fmt.Errorf("OIDC login stuck on IdP: no form and no JS redirect (url: %s)", resp.Request.URL.String())
			}
			slog.Debug("oidc: following JS redirect", "attempt", attempt, "source", source, "url", nextURL)
			resp, body, err = o.doGet(nextURL)
			if err != nil {
				return fmt.Errorf("OIDC follow %s: %w", source, err)
			}
			continue
		}

		actionURL := extractFormAction(body)
		if actionURL == "" {
			if resp.StatusCode == 200 && atZuul {
				return nil
			}
			return fmt.Errorf("no form found (status %d, url: %s)", resp.StatusCode, resp.Request.URL.String())
		}

		actionURL = resolveURL(resp.Request.URL, actionURL)
		formData := extractHiddenFields(body)
		isLogin := isLoginForm(body)
		if isLogin {
			formData.Set("username", o.username)
			formData.Set("password", o.password)
		}

		slog.Debug("oidc: submitting form", "attempt", attempt, "action", actionURL, "isLogin", isLogin)

		resp, body, err = o.doPost(actionURL, formData)
		if err != nil {
			return fmt.Errorf("form submit (attempt %d): %w", attempt, err)
		}

		slog.Debug("oidc: form result", "attempt", attempt, "status", resp.StatusCode, "url", resp.Request.URL.String(), "hasForm", containsForm(body))

		if strings.Contains(body, "Invalid username or password") ||
			strings.Contains(body, "invalid_grant") {
			return fmt.Errorf("invalid username or password")
		}
	}

	return fmt.Errorf("too many form redirects during OIDC login")
}

func (o *OIDC) doGet(rawURL string) (*http.Response, string, error) {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := o.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, string(b), nil
}

func (o *OIDC) doPost(rawURL string, data url.Values) (*http.Response, string, error) {
	req, err := http.NewRequest("POST", rawURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := o.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, string(b), nil
}

// browserTransport wraps an http.RoundTripper and adds browser-like headers
// to every request. mod_auth_openidc uses Sec-Fetch-Mode/Sec-Fetch-Dest to
// distinguish browser navigations (→ redirect to IdP) from API/XHR calls
// (→ return 401). Without these headers the module treats the request as
// non-browser and never starts the OIDC flow.
type browserTransport struct {
	base http.RoundTripper
}

func (t *browserTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36")
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	}
	if req.Header.Get("Sec-Fetch-Mode") == "" {
		req.Header.Set("Sec-Fetch-Mode", "navigate")
	}
	if req.Header.Get("Sec-Fetch-Dest") == "" {
		req.Header.Set("Sec-Fetch-Dest", "document")
	}
	if req.Header.Get("Sec-Fetch-Site") == "" {
		req.Header.Set("Sec-Fetch-Site", "none")
	}
	if req.Header.Get("Sec-Fetch-User") == "" {
		req.Header.Set("Sec-Fetch-User", "?1")
	}
	return t.base.RoundTrip(req)
}

func containsForm(body string) bool {
	return strings.Contains(body, "<form") || strings.Contains(body, "<FORM") ||
		strings.Contains(body, "<Form")
}

func isLoginForm(body string) bool {
	return strings.Contains(body, `name="username"`) || strings.Contains(body, `id="username"`) ||
		strings.Contains(body, `name="password"`) || strings.Contains(body, `id="password"`)
}

var formActionRegex = regexp.MustCompile(`(?i)<form[^>]*action="([^"]+)"`)
var inputRegex = regexp.MustCompile(`(?i)<input[^>]*/?>`)
var attrNameRegex = regexp.MustCompile(`(?i)name="([^"]+)"`)
var attrValueRegex = regexp.MustCompile(`(?i)value="([^"]*)"`)
var attrTypeRegex = regexp.MustCompile(`(?i)type="([^"]+)"`)
var replaceStateRegex = regexp.MustCompile(`history\.replaceState\([^,]*,\s*"[^"]*",\s*"([^"]+)"\)`)
var anchorHrefRegex = regexp.MustCompile(`(?i)<a[^>]*\bhref="([^"]+)"[^>]*>`)

func extractFormAction(body string) string {
	matches := formActionRegex.FindStringSubmatch(body)
	if len(matches) < 2 {
		return ""
	}
	return html.UnescapeString(matches[1])
}

// extractHiddenFields finds all <input type="hidden"> in the HTML and returns
// their name/value pairs. This is essential for following OIDC callback forms
// that carry authorization codes, state tokens, etc.
func extractHiddenFields(body string) url.Values {
	values := url.Values{}
	for _, input := range inputRegex.FindAllString(body, -1) {
		tm := attrTypeRegex.FindStringSubmatch(input)
		if len(tm) < 2 || !strings.EqualFold(tm[1], "hidden") {
			continue
		}
		nm := attrNameRegex.FindStringSubmatch(input)
		if len(nm) < 2 {
			continue
		}
		val := ""
		vm := attrValueRegex.FindStringSubmatch(input)
		if len(vm) >= 2 {
			val = html.UnescapeString(vm[1])
		}
		values.Set(nm[1], val)
	}
	return values
}

func extractReplaceStateURL(body string) string {
	m := replaceStateRegex.FindStringSubmatch(body)
	if len(m) < 2 {
		return ""
	}
	return html.UnescapeString(m[1])
}

func extractLinkHref(body string) string {
	m := anchorHrefRegex.FindStringSubmatch(body)
	if len(m) < 2 {
		return ""
	}
	return html.UnescapeString(m[1])
}

func resolveURL(base *url.URL, ref string) string {
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return ref
	}
	parsed, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return base.ResolveReference(parsed).String()
}
