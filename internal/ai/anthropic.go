package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os/exec"
	"strings"

	"github.com/Valkyrie00/hzuul/internal/config"
)

const (
	defaultAnthropicModel = "claude-sonnet-4-20250514"
	defaultVertexModel    = "claude-opus-4-6"
	defaultVertexRegion   = "us-east5"
	anthropicEndpoint     = "https://api.anthropic.com/v1/messages"
)

// AnthropicProvider implements Provider for the Anthropic Messages API.
// It supports both the direct Anthropic endpoint and Google Vertex AI,
// which exposes the same API format under a different endpoint + auth.
type AnthropicProvider struct {
	endpoint   string
	authHeader string
	authValue  string
	model      string
	name       string
	isVertex   bool
	vertexCfg  *vertexConfig // non-nil only for Vertex AI, enables token refresh
	httpClient *http.Client
}

type vertexConfig struct {
	project string
	region  string
}

type anthropicRequest struct {
	Model            string             `json:"model,omitempty"`
	AnthropicVersion string             `json:"anthropic_version,omitempty"`
	MaxTokens        int                `json:"max_tokens"`
	System           string             `json:"system,omitempty"`
	Messages         []anthropicMessage `json:"messages"`
	Stream           bool               `json:"stream"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// NewAnthropicDirect creates a provider using the Anthropic API directly.
// Only activates if anthropic_api_key is explicitly set in config.
func NewAnthropicDirect(cfg config.AIConfig) *AnthropicProvider {
	key := cfg.AnthropicAPIKey
	if key == "" {
		return nil
	}
	model := cfg.Model
	if model == "" {
		model = defaultAnthropicModel
	}
	return &AnthropicProvider{
		endpoint:   anthropicEndpoint,
		authHeader: "x-api-key",
		authValue:  key,
		model:      model,
		name:       "Anthropic",
		httpClient: &http.Client{},
	}
}

// NewVertexAI creates a provider using Claude via Google Vertex AI.
// Only activates if vertex_project_id is explicitly set in config.
func NewVertexAI(cfg config.AIConfig) *AnthropicProvider {
	project := cfg.VertexProjectID
	if project == "" {
		return nil
	}
	token, err := gcloudAccessToken()
	if err != nil || token == "" {
		slog.Debug("vertex AI disabled: no gcloud application-default token", "err", err)
		return nil
	}
	region := cfg.VertexRegion
	if region == "" {
		region = defaultVertexRegion
	}
	model := cfg.Model
	if model == "" {
		model = defaultVertexModel
	}
	endpoint := fmt.Sprintf(
		"https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/anthropic/models/%s:streamRawPredict",
		region, project, region, model,
	)
	return &AnthropicProvider{
		endpoint:   endpoint,
		authHeader: "Authorization",
		authValue:  "Bearer " + token,
		model:      model,
		name:       "Vertex AI",
		isVertex:   true,
		vertexCfg:  &vertexConfig{project: project, region: region},
		httpClient: &http.Client{},
	}
}

func (p *AnthropicProvider) Name() string  { return p.name }
func (p *AnthropicProvider) Model() string { return p.model }

func (p *AnthropicProvider) refreshVertexToken() error {
	if p.vertexCfg == nil {
		return nil
	}
	token, err := gcloudAccessToken()
	if err != nil {
		return fmt.Errorf("refreshing gcloud token: %w", err)
	}
	p.authValue = "Bearer " + token
	return nil
}

func (p *AnthropicProvider) Stream(system, user string, onChunk func(string)) error {
	if err := p.refreshVertexToken(); err != nil {
		return err
	}

	reqBody := anthropicRequest{
		MaxTokens: 4096,
		System:    system,
		Messages: []anthropicMessage{
			{Role: "user", Content: user},
		},
		Stream: true,
	}

	if p.isVertex {
		reqBody.AnthropicVersion = "vertex-2023-10-16"
	} else {
		reqBody.Model = p.model
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", p.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(p.authHeader, p.authValue)
	if !p.isVertex {
		req.Header.Set("anthropic-version", "2023-06-01")
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s API error %d: %s", p.name, resp.StatusCode, formatAPIError(resp.StatusCode, string(respBody), p.model))
	}

	return ParseSSE(resp.Body, onChunk)
}

func formatAPIError(status int, body, model string) string {
	switch status {
	case 401:
		return "authentication failed — check your API key or gcloud credentials"
	case 403:
		return "access denied — your account may not have permission for this model"
	case 404:
		return fmt.Sprintf("model %q not found — check the model name in your config", model)
	case 429:
		return "rate limit exceeded — wait a moment and try again"
	case 500, 502, 503:
		return "provider is temporarily unavailable — try again later"
	}
	// Try to extract JSON error message
	var errResp struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal([]byte(body), &errResp) == nil {
		if errResp.Error.Message != "" {
			return errResp.Error.Message
		}
		if errResp.Message != "" {
			return errResp.Message
		}
	}
	if len(body) > 200 {
		return body[:200] + "..."
	}
	return body
}

func gcloudAccessToken() (string, error) {
	out, err := exec.Command("gcloud", "auth", "application-default", "print-access-token").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
