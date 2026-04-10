package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestActive_Found(t *testing.T) {
	cfg := &Config{
		CurrentContext: "prod",
		Contexts: map[string]*Context{
			"prod": {URL: "https://zuul.example.com", Tenant: "main"},
		},
	}
	ctx, err := cfg.Active()
	if err != nil {
		t.Fatal(err)
	}
	if ctx.URL != "https://zuul.example.com" {
		t.Errorf("URL = %q", ctx.URL)
	}
}

func TestActive_NotFound(t *testing.T) {
	cfg := &Config{
		CurrentContext: "missing",
		Contexts:       map[string]*Context{},
	}
	_, err := cfg.Active()
	if err == nil {
		t.Fatal("expected error for missing context")
	}
}

func TestSSLVerify_DefaultTrue(t *testing.T) {
	ctx := &Context{}
	if !ctx.SSLVerify() {
		t.Error("expected true when VerifySSL is nil")
	}
}

func TestSSLVerify_ExplicitFalse(t *testing.T) {
	f := false
	ctx := &Context{VerifySSL: &f}
	if ctx.SSLVerify() {
		t.Error("expected false when VerifySSL is explicitly false")
	}
}

func TestSSLVerify_ExplicitTrue(t *testing.T) {
	tr := true
	ctx := &Context{VerifySSL: &tr}
	if !ctx.SSLVerify() {
		t.Error("expected true when VerifySSL is explicitly true")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := defaultConfig()
	if cfg.CurrentContext != "default" {
		t.Errorf("CurrentContext = %q", cfg.CurrentContext)
	}
	ctx, ok := cfg.Contexts["default"]
	if !ok {
		t.Fatal("missing default context")
	}
	if ctx.URL != "https://zuul.opendev.org" {
		t.Errorf("URL = %q", ctx.URL)
	}
	if ctx.Tenant != "openstack" {
		t.Errorf("Tenant = %q", ctx.Tenant)
	}
}

func TestLoadSave_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := &Config{
		CurrentContext: "test",
		Contexts: map[string]*Context{
			"test": {
				URL:    "https://zuul.test.com",
				Tenant: "testing",
				Auth:   "none",
			},
		},
		AI: AIConfig{
			Provider:        "vertex",
			VertexProjectID: "my-project",
			VertexRegion:    "us-east5",
		},
	}

	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.CurrentContext != "test" {
		t.Errorf("CurrentContext = %q", loaded.CurrentContext)
	}
	ctx, err := loaded.Active()
	if err != nil {
		t.Fatal(err)
	}
	if ctx.URL != "https://zuul.test.com" {
		t.Errorf("URL = %q", ctx.URL)
	}
	if loaded.AI.Provider != "vertex" {
		t.Errorf("AI.Provider = %q", loaded.AI.Provider)
	}
	if loaded.AI.VertexProjectID != "my-project" {
		t.Errorf("AI.VertexProjectID = %q", loaded.AI.VertexProjectID)
	}
}

func TestLoad_CreatesDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "config.yaml")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.CurrentContext != "default" {
		t.Errorf("CurrentContext = %q", cfg.CurrentContext)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected default config file to be created")
	}
}
