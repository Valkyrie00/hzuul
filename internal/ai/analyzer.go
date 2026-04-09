package ai

import (
	"strings"

	"github.com/Valkyrie00/hzuul/internal/config"
)

// Analyzer wraps any Provider and exposes async streaming.
type Analyzer struct {
	provider Provider
}

// NewAnalyzer creates an Analyzer by selecting the best available provider
// based on the config. Provider resolution order for "auto":
//  1. Vertex AI (Claude via GCP)
//  2. Anthropic (direct API)
//
// This factory is the only place where providers are wired up.
// To add a new vendor, implement Provider and add a case here.
func NewAnalyzer(cfg config.AIConfig) *Analyzer {
	p := resolveProvider(cfg)
	if p == nil {
		return nil
	}
	return &Analyzer{provider: p}
}

// HasAnalyzer checks if at least one provider can be configured.
func HasAnalyzer(cfg config.AIConfig) bool {
	return resolveProvider(cfg) != nil
}

// ProviderLabel returns a short label for the active provider, or empty if none.
func ProviderLabel(cfg config.AIConfig) string {
	p := resolveProvider(cfg)
	if p != nil {
		return p.Name()
	}
	// Config points at Vertex but gcloud ADC failed — YAML was still applied.
	switch strings.ToLower(cfg.Provider) {
	case "vertex":
		if cfg.VertexProjectID != "" {
			return "Vertex (gcloud ADC?)"
		}
	case "gemini-vertex":
		if cfg.VertexProjectID != "" {
			return "Gemini Vertex (gcloud ADC?)"
		}
	}
	return ""
}

func resolveProvider(cfg config.AIConfig) Provider {
	provider := strings.ToLower(cfg.Provider)
	if provider == "" {
		provider = "auto"
	}

	factories := providerOrder(provider)
	for _, factory := range factories {
		if p := factory(cfg); p != nil {
			return p
		}
	}
	return nil
}

type providerFactory func(config.AIConfig) Provider

func providerOrder(provider string) []providerFactory {
	switch provider {
	case "vertex":
		return []providerFactory{vertexFactory}
	case "anthropic":
		return []providerFactory{anthropicFactory}
	case "gemini":
		return []providerFactory{geminiDirectFactory}
	case "gemini-vertex":
		return []providerFactory{geminiVertexFactory}
	default: // "auto"
		return []providerFactory{vertexFactory, anthropicFactory, geminiVertexFactory, geminiDirectFactory}
	}
}

func vertexFactory(cfg config.AIConfig) Provider {
	if p := NewVertexAI(cfg); p != nil {
		return p
	}
	return nil
}

func anthropicFactory(cfg config.AIConfig) Provider {
	if p := NewAnthropicDirect(cfg); p != nil {
		return p
	}
	return nil
}

func geminiDirectFactory(cfg config.AIConfig) Provider {
	if p := NewGeminiDirect(cfg); p != nil {
		return p
	}
	return nil
}

func geminiVertexFactory(cfg config.AIConfig) Provider {
	if p := NewGeminiVertex(cfg); p != nil {
		return p
	}
	return nil
}

// ProviderName returns the active provider's display name.
func (a *Analyzer) ProviderName() string { return a.provider.Name() }

// ModelName returns the active model identifier.
func (a *Analyzer) ModelName() string { return a.provider.Model() }

// Analyze sends the prompt to the AI provider and streams the response.
func (a *Analyzer) Analyze(systemPrompt, userPrompt string, onChunk func(string), onDone func(error)) {
	go func() {
		err := a.provider.Stream(systemPrompt, userPrompt, onChunk)
		onDone(err)
	}()
}
