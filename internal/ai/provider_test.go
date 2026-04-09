package ai

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Valkyrie00/hzuul/internal/config"
)

func TestAnthropicDirect_Stream(t *testing.T) {
	sseBody := `data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}

data: {"type":"content_block_delta","delta":{"type":"text_delta","text":" AI"}}

data: [DONE]
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-key" {
			t.Error("missing API key header")
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Error("missing anthropic-version header")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(sseBody))
	}))
	defer srv.Close()

	p := &AnthropicProvider{
		endpoint:   srv.URL,
		authHeader: "x-api-key",
		authValue:  "test-key",
		model:      "test-model",
		name:       "Test",
		httpClient: srv.Client(),
	}

	var chunks []string
	err := p.Stream("system", "user", func(s string) {
		chunks = append(chunks, s)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 2 || chunks[0] != "Hello" || chunks[1] != " AI" {
		t.Errorf("chunks = %v", chunks)
	}
}

func TestAnthropicDirect_StreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"error":{"message":"invalid key"}}`))
	}))
	defer srv.Close()

	p := &AnthropicProvider{
		endpoint:   srv.URL,
		authHeader: "x-api-key",
		authValue:  "bad",
		model:      "m",
		name:       "Test",
		httpClient: srv.Client(),
	}

	err := p.Stream("s", "u", func(string) {})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("got %v", err)
	}
}

func TestVertexProvider_Stream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Error("missing Bearer token")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"vertex"}}

data: [DONE]
`))
	}))
	defer srv.Close()

	p := &AnthropicProvider{
		endpoint:   srv.URL,
		authHeader: "Authorization",
		authValue:  "Bearer fake-token",
		model:      "claude-test",
		name:       "Vertex",
		isVertex:   true,
		httpClient: srv.Client(),
	}

	var chunks []string
	err := p.Stream("s", "u", func(s string) { chunks = append(chunks, s) })
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 || chunks[0] != "vertex" {
		t.Errorf("chunks = %v", chunks)
	}
}

func TestGeminiDirect_Stream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(`data: {"candidates":[{"content":{"parts":[{"text":"gemini"}]}}]}

data: [DONE]
`))
	}))
	defer srv.Close()

	p := &GeminiProvider{
		endpoint:   srv.URL,
		apiKey:     "test-key",
		model:      "gemini-test",
		provName:   "Gemini",
		httpClient: srv.Client(),
	}

	var chunks []string
	err := p.Stream("s", "u", func(s string) { chunks = append(chunks, s) })
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 || chunks[0] != "gemini" {
		t.Errorf("chunks = %v", chunks)
	}
}

func TestGeminiDirect_StreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		w.Write([]byte("forbidden"))
	}))
	defer srv.Close()

	p := &GeminiProvider{
		endpoint:   srv.URL,
		model:      "m",
		provName:   "Gemini",
		httpClient: srv.Client(),
	}

	err := p.Stream("s", "u", func(string) {})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewAnthropicDirect_NoKey(t *testing.T) {
	p := NewAnthropicDirect(config.AIConfig{})
	if p != nil {
		t.Error("expected nil without API key")
	}
}

func TestNewAnthropicDirect_WithKey(t *testing.T) {
	p := NewAnthropicDirect(config.AIConfig{AnthropicAPIKey: "sk-test"})
	if p == nil {
		t.Fatal("expected non-nil")
	}
	if p.Name() != "Anthropic" {
		t.Errorf("name = %q", p.Name())
	}
	if p.Model() != defaultAnthropicModel {
		t.Errorf("model = %q", p.Model())
	}
}

func TestNewAnthropicDirect_ModelOverride(t *testing.T) {
	p := NewAnthropicDirect(config.AIConfig{AnthropicAPIKey: "sk-test", Model: "custom-model"})
	if p.Model() != "custom-model" {
		t.Errorf("model = %q", p.Model())
	}
}

func TestNewGeminiDirect_NoKey(t *testing.T) {
	p := NewGeminiDirect(config.AIConfig{})
	if p != nil {
		t.Error("expected nil without API key")
	}
}

func TestNewGeminiDirect_WithKey(t *testing.T) {
	p := NewGeminiDirect(config.AIConfig{GeminiAPIKey: "AIza-test"})
	if p == nil {
		t.Fatal("expected non-nil")
	}
	if p.Name() != "Gemini" {
		t.Errorf("name = %q", p.Name())
	}
	if p.Model() != defaultGeminiModel {
		t.Errorf("model = %q", p.Model())
	}
}

func TestAnthropicDirect_StreamPartialThenError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		w.Write([]byte(`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"partial"}}` + "\n\n"))
		flusher.Flush()
		// abruptly close without [DONE]
	}))
	defer srv.Close()

	p := &AnthropicProvider{
		endpoint:   srv.URL,
		authHeader: "x-api-key",
		authValue:  "k",
		model:      "m",
		name:       "Test",
		httpClient: srv.Client(),
	}

	var chunks []string
	err := p.Stream("s", "u", func(s string) { chunks = append(chunks, s) })
	// should still have received the partial chunk
	if len(chunks) != 1 || chunks[0] != "partial" {
		t.Errorf("expected partial chunk, got %v (err=%v)", chunks, err)
	}
}

func TestGeminiDirect_StreamMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {not json}\ndata: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"ok\"}]}}]}\ndata: [DONE]\n"))
	}))
	defer srv.Close()

	p := &GeminiProvider{
		endpoint:   srv.URL,
		model:      "m",
		provName:   "Gemini",
		httpClient: srv.Client(),
	}

	var chunks []string
	err := p.Stream("s", "u", func(s string) { chunks = append(chunks, s) })
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 || chunks[0] != "ok" {
		t.Errorf("malformed line should be skipped, valid line should work: chunks=%v", chunks)
	}
}

func TestVertexProvider_SetsAnthropicVersion(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)
		w.Write([]byte("data: [DONE]\n"))
	}))
	defer srv.Close()

	p := &AnthropicProvider{
		endpoint:   srv.URL,
		authHeader: "Authorization",
		authValue:  "Bearer token",
		model:      "claude-test",
		name:       "Vertex",
		isVertex:   true,
		httpClient: srv.Client(),
	}

	p.Stream("sys", "usr", func(string) {})

	if gotBody["anthropic_version"] != "vertex-2023-10-16" {
		t.Errorf("anthropic_version = %v", gotBody["anthropic_version"])
	}
	if gotBody["model"] != nil {
		t.Errorf("vertex requests should NOT include model in body, got %v", gotBody["model"])
	}
}

func TestAnthropicDirect_SetsModelInBody(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)
		w.Write([]byte("data: [DONE]\n"))
	}))
	defer srv.Close()

	p := &AnthropicProvider{
		endpoint:   srv.URL,
		authHeader: "x-api-key",
		authValue:  "key",
		model:      "claude-sonnet",
		name:       "Anthropic",
		isVertex:   false,
		httpClient: srv.Client(),
	}

	p.Stream("sys", "usr", func(string) {})

	if gotBody["model"] != "claude-sonnet" {
		t.Errorf("model = %v, want claude-sonnet", gotBody["model"])
	}
	if gotBody["anthropic_version"] != nil {
		t.Errorf("direct requests should NOT include anthropic_version, got %v", gotBody["anthropic_version"])
	}
}

func TestProviderOrder(t *testing.T) {
	tests := []struct {
		name string
		want int
	}{
		{"vertex", 1},
		{"anthropic", 1},
		{"gemini", 1},
		{"gemini-vertex", 1},
		{"auto", 4},
		{"", 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := providerOrder(tt.name)
			if len(got) != tt.want {
				t.Errorf("providerOrder(%q) = %d factories, want %d", tt.name, len(got), tt.want)
			}
		})
	}
}
