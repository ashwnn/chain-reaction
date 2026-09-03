package llm

import (
	"time"
)

const defaultGroqBaseURL = "https://api.groq.com/openai/v1"

// newGroqProvider creates an OpenAI-compatible provider with Groq defaults.
// Groq's API is fully OpenAI-compatible.
func newGroqProvider(cfg ProviderConfig, timeout time.Duration) (*openaiProvider, error) {
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultGroqBaseURL
	}
	return newOpenAIProvider(cfg, timeout)
}
