package main

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// ErrOverloaded is returned when an LLM API is overloaded (retryable).
var ErrOverloaded = fmt.Errorf("LLM API overloaded")

// StreamResult holds the text and extracted JSON from a streaming LLM call.
type StreamResult struct {
	RawText    string
	ResultJSON string
}

// ModelDef describes a model in a fallback chain.
type ModelDef struct {
	ID   string
	Name string
}

// LLMProvider is the interface all LLM providers implement.
type LLMProvider interface {
	// Call makes a non-streaming LLM call. Returns the text response.
	Call(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error)

	// Stream makes a streaming SSE call, forwarding progress events to the client.
	Stream(ctx context.Context, apiKey string, body []byte, w http.ResponseWriter, flusher http.Flusher) (*StreamResult, error)

	// BuildStreamBody constructs the provider-specific JSON request body.
	BuildStreamBody(model string, maxTokens int, prompt string, useWebSearch bool) ([]byte, error)

	// VerifyKey checks if an API key is valid. Returns "active", "invalid", "no_credits", or "error".
	VerifyKey(ctx context.Context, apiKey string) (string, error)

	// Models returns the model fallback chain (primary first, then fallbacks).
	Models() []ModelDef

	// SmallModel returns the cheapest/fastest model for lightweight tasks (e.g. video assessments).
	SmallModel() string

	// Name returns the display name (e.g. "Anthropic", "OpenAI").
	Name() string

	// ProviderID returns the provider identifier (e.g. "anthropic", "openai").
	ProviderID() string
}

// llmHTTPClient is a shared HTTP client for non-streaming LLM provider API calls.
var llmHTTPClient = &http.Client{Timeout: 120 * time.Second}

// llmStreamClient is used for streaming LLM calls where the response is read
// incrementally. No timeout — cancellation is handled by the request context
// (e.g. client disconnect).
var llmStreamClient = &http.Client{}

// providers is the global registry of available LLM providers.
var providers = map[string]LLMProvider{}

func init() {
	anthropic := &AnthropicProvider{}
	openai := &OpenAIProvider{}
	grok := &GrokProvider{}
	gemini := &GeminiProvider{}

	providers["anthropic"] = anthropic
	providers["openai"] = openai
	providers["grok"] = grok
	providers["gemini"] = gemini
}

// getProvider returns the LLMProvider for the given provider ID, or nil if unknown.
func getProvider(id string) LLMProvider {
	return providers[id]
}

// validProviderIDs returns the list of supported provider identifiers.
// Includes "youtube" which is not an LLM provider but uses the same key storage.
func validProviderIDs() []string {
	return []string{"anthropic", "openai", "grok", "gemini", "youtube"}
}
