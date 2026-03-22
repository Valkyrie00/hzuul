package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	CurrentContext string              `yaml:"current_context"`
	Contexts       map[string]*Context `yaml:"contexts"`
}

type Context struct {
	URL       string `yaml:"url"`
	Tenant    string `yaml:"tenant"`
	Auth      string `yaml:"auth"`      // "oidc", "kerberos", or "none"
	VerifySSL *bool  `yaml:"verify_ssl,omitempty"`
	CACert    string `yaml:"ca_cert,omitempty"` // path to CA bundle (e.g. tls-ca-bundle.pem)
	Username  string `yaml:"username,omitempty"` // for auth: oidc

	// password is not persisted in YAML -- set at runtime via env or prompt
	password string `yaml:"-"` //nolint:unused
}

func (c *Context) SetPassword(p string) {
	c.password = p
}

func (c *Context) Password() string {
	return c.password
}

func (c *Config) Active() (*Context, error) {
	ctx, ok := c.Contexts[c.CurrentContext]
	if !ok {
		return nil, fmt.Errorf("context %q not found in config", c.CurrentContext)
	}
	return ctx, nil
}

func (c *Context) SSLVerify() bool {
	if c.VerifySSL == nil {
		return true
	}
	return *c.VerifySSL
}

func defaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "hzuul", "config.yaml")
}

func Load(path string) (*Config, error) {
	if path == "" {
		path = defaultPath()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultConfig(), nil
		}
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}

	if cfg.Contexts == nil {
		cfg.Contexts = make(map[string]*Context)
	}

	return &cfg, nil
}

func (c *Config) Save(path string) error {
	if path == "" {
		path = defaultPath()
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	return os.WriteFile(path, data, 0o644)
}

func defaultConfig() *Config {
	return &Config{
		CurrentContext: "default",
		Contexts: map[string]*Context{
			"default": {
				URL:    "https://zuul.opendev.org",
				Tenant: "openstack",
				Auth:   "none",
			},
		},
	}
}
