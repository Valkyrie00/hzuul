package ai

// Provider is the interface that all AI backends must implement.
// Adding a new vendor (Ollama, OpenAI, Gemini, etc.) only requires
// implementing this interface and registering a factory in analyzer.go.
type Provider interface {
	// Stream sends system+user prompts and calls onChunk for each token.
	Stream(systemPrompt, userPrompt string, onChunk func(string)) error

	// Name returns the provider display name (e.g. "Vertex AI", "Anthropic", "Ollama").
	Name() string

	// Model returns the model identifier being used.
	Model() string
}
