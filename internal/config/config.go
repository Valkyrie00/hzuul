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
	AI             AIConfig            `yaml:"ai,omitempty"`
}

type AIConfig struct {
	Provider        string `yaml:"provider,omitempty"`         // "vertex", "anthropic", or "auto" (default)
	Model           string `yaml:"model,omitempty"`            // model override
	AnthropicAPIKey string `yaml:"anthropic_api_key,omitempty"`
	VertexProjectID string `yaml:"vertex_project_id,omitempty"`
	VertexRegion    string `yaml:"vertex_region,omitempty"`    // default: us-east5
}

type Context struct {
	URL       string `yaml:"url"`
	Tenant    string `yaml:"tenant"`
	Auth      string `yaml:"auth"`                // "kerberos" or "none"
	VerifySSL *bool  `yaml:"verify_ssl,omitempty"`
	CACert    string `yaml:"ca_cert,omitempty"` // path to CA bundle (e.g. tls-ca-bundle.pem)
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
	return filepath.Join(DataDir(), "config.yaml")
}

// DataDir returns the base directory for all hzuul data (~/.hzuul/).
func DataDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".hzuul")
}

func Load(path string) (*Config, error) {
	if path == "" {
		path = defaultPath()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := defaultConfig()
			_ = cfg.SaveWithDefaults(path)
			return cfg, nil
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

func (c *Config) SaveWithDefaults(path string) error {
	if err := c.Save(path); err != nil {
		return err
	}
	if path == "" {
		path = defaultPath()
	}
	if c.AI.Provider == "" && c.AI.AnthropicAPIKey == "" && c.AI.VertexProjectID == "" {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return nil
		}
		defer f.Close()
		f.WriteString(aiConfigComment)
	}
	return nil
}

const aiConfigComment = `
# AI-powered build failure analysis (optional)
# Uncomment ONE of the providers below:
#
# Anthropic API (direct):
# ai:
#   provider: anthropic
#   anthropic_api_key: sk-ant-...
#
# Google Vertex AI (Claude via GCP):
# ai:
#   provider: vertex
#   vertex_project_id: my-gcp-project
#   vertex_region: us-east5
#
# model: claude-sonnet-4-20250514  # optional model override
`

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
