package ai

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Valkyrie00/hzuul/internal/config"
)

const (
	defaultGeminiModel    = "gemini-2.5-pro"
	geminiStudioEndpoint  = "https://generativelanguage.googleapis.com/v1beta/models"
)

// GeminiProvider implements Provider for Google's Gemini API.
// Supports both Google AI Studio (API key) and Vertex AI (gcloud auth).
type GeminiProvider struct {
	endpoint   string
	apiKey     string
	authToken  string
	isVertex   bool
	model      string
	provName   string
	vertexCfg  *vertexConfig
	httpClient *http.Client
}

type geminiRequest struct {
	Contents         []geminiContent  `json:"contents"`
	SystemInstruction *geminiContent  `json:"systemInstruction,omitempty"`
	GenerationConfig geminiGenConfig  `json:"generationConfig"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenConfig struct {
	MaxOutputTokens int `json:"maxOutputTokens"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

// NewGeminiDirect creates a provider using the Google AI Studio API.
// Only activates if gemini_api_key is explicitly set in config.
func NewGeminiDirect(cfg config.AIConfig) *GeminiProvider {
	key := cfg.GeminiAPIKey
	if key == "" {
		return nil
	}
	model := cfg.Model
	if model == "" {
		model = defaultGeminiModel
	}
	endpoint := fmt.Sprintf("%s/%s:streamGenerateContent?alt=sse&key=%s",
		geminiStudioEndpoint, model, key)

	return &GeminiProvider{
		endpoint:   endpoint,
		apiKey:     key,
		model:      model,
		provName:   "Gemini",
		httpClient: &http.Client{},
	}
}

// NewGeminiVertex creates a provider using Gemini via Google Vertex AI.
// Only activates if vertex_project_id is set and provider is "gemini-vertex".
func NewGeminiVertex(cfg config.AIConfig) *GeminiProvider {
	project := cfg.VertexProjectID
	if project == "" {
		return nil
	}
	token, err := gcloudAccessToken()
	if err != nil || token == "" {
		return nil
	}
	region := cfg.VertexRegion
	if region == "" {
		region = defaultVertexRegion
	}
	model := cfg.Model
	if model == "" {
		model = defaultGeminiModel
	}
	endpoint := fmt.Sprintf(
		"https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:streamGenerateContent?alt=sse",
		region, project, region, model,
	)
	return &GeminiProvider{
		endpoint:   endpoint,
		authToken:  token,
		isVertex:   true,
		model:      model,
		provName:   "Gemini (Vertex AI)",
		vertexCfg:  &vertexConfig{project: project, region: region},
		httpClient: &http.Client{},
	}
}

func (p *GeminiProvider) Name() string  { return p.provName }
func (p *GeminiProvider) Model() string { return p.model }

func (p *GeminiProvider) refreshToken() error {
	if p.vertexCfg == nil {
		return nil
	}
	token, err := gcloudAccessToken()
	if err != nil {
		return fmt.Errorf("refreshing gcloud token: %w", err)
	}
	p.authToken = token
	return nil
}

func (p *GeminiProvider) Stream(system, user string, onChunk func(string)) error {
	if err := p.refreshToken(); err != nil {
		return err
	}

	reqBody := geminiRequest{
		Contents: []geminiContent{
			{Role: "user", Parts: []geminiPart{{Text: user}}},
		},
		GenerationConfig: geminiGenConfig{MaxOutputTokens: 8192},
	}
	if system != "" {
		reqBody.SystemInstruction = &geminiContent{
			Parts: []geminiPart{{Text: system}},
		}
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
	if p.isVertex {
		req.Header.Set("Authorization", "Bearer "+p.authToken)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Gemini API error %d: %s", resp.StatusCode, formatAPIError(resp.StatusCode, string(respBody), p.model))
	}

	return parseGeminiSSE(resp.Body, onChunk)
}

func parseGeminiSSE(r io.Reader, onChunk func(string)) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var resp geminiResponse
		if err := json.Unmarshal([]byte(data), &resp); err != nil {
			continue
		}

		if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
			text := resp.Candidates[0].Content.Parts[0].Text
			if text != "" {
				onChunk(text)
			}
		}
	}
	return scanner.Err()
}

