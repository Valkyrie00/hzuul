package api

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/vcastell/hzuul/internal/auth"
	"github.com/vcastell/hzuul/internal/config"
)

type Client struct {
	doer    auth.HTTPDoer
	baseURL string
	tenant  string
}

func NewClient(ctx *config.Context) (*Client, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !ctx.SSLVerify(),
		},
	}

	var authProvider auth.Provider
	switch strings.ToLower(ctx.Auth) {
	case "oidc":
		if ctx.Username == "" {
			return nil, fmt.Errorf("oidc auth requires 'username' in config")
		}
		password := ctx.Password()
		if password == "" {
			return nil, fmt.Errorf("oidc auth: password not provided (set via prompt or HZUUL_PASSWORD env)")
		}
		o, err := auth.NewOIDC(strings.TrimRight(ctx.URL, "/"), ctx.Username, password, ctx.SSLVerify(), ctx.CACert)
		if err != nil {
			return nil, fmt.Errorf("oidc auth: %w", err)
		}
		authProvider = o
	case "kerberos":
		baseURL := strings.TrimRight(ctx.URL, "/")
		k, err := auth.NewKerberos(baseURL+"/api/tenants", ctx.SSLVerify(), ctx.CACert)
		if err != nil {
			return nil, fmt.Errorf("kerberos auth: %w", err)
		}
		authProvider = k
	default:
		authProvider = &auth.NoAuth{}
	}

	doer := authProvider.HTTPClient(transport)

	// Set timeout on the underlying http.Client if possible
	if hc, ok := doer.(*http.Client); ok {
		hc.Timeout = 30 * time.Second
	}

	return &Client{
		doer:    doer,
		baseURL: strings.TrimRight(ctx.URL, "/"),
		tenant:  ctx.Tenant,
	}, nil
}

func (c *Client) Tenant() string {
	return c.tenant
}

func (c *Client) SetTenant(t string) {
	c.tenant = t
}

func (c *Client) BuildURL(uuid string) string {
	return fmt.Sprintf("%s/t/%s/build/%s", c.baseURL, c.tenant, uuid)
}

func (c *Client) ProjectURL(canonicalName string) string {
	return fmt.Sprintf("%s/t/%s/project/%s", c.baseURL, c.tenant, canonicalName)
}

func (c *Client) JobURL(name string) string {
	return fmt.Sprintf("%s/t/%s/job/%s", c.baseURL, c.tenant, name)
}

func (c *Client) get(path string, params url.Values) (*http.Response, error) {
	u := c.baseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request for %s: %w", path, err)
	}

	resp, err := c.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", path, err)
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("GET %s: %s — %s", path, resp.Status, string(body))
	}

	return resp, nil
}

func (c *Client) getJSON(path string, params url.Values, target any) error {
	resp, err := c.get(path, params)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return json.NewDecoder(resp.Body).Decode(target)
}

func (c *Client) postJSON(path string, body any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}

	u := c.baseURL + path
	req, err := http.NewRequest("POST", u, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doer.Do(req)
	if err != nil {
		return fmt.Errorf("POST %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("POST %s: %s", path, resp.Status)
	}
	return nil
}

func (c *Client) delete(path string) error {
	u := c.baseURL + path
	req, err := http.NewRequest("DELETE", u, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.doer.Do(req)
	if err != nil {
		return fmt.Errorf("DELETE %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("DELETE %s: %s", path, resp.Status)
	}
	return nil
}

func (c *Client) tenantPath(suffix string) string {
	return fmt.Sprintf("/api/tenant/%s/%s", c.tenant, suffix)
}
