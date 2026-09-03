package gateway

// ProviderTranslator adapts between OpenAI-compatible format and a provider's
// native API format. Each provider that isn't OpenAI-compatible needs its own
// implementation.
type ProviderTranslator interface {
	// TransformRequest converts an OpenAI-format request body into the
	// provider's native request format.
	TransformRequest(body []byte, model string) ([]byte, error)

	// TransformResponse converts a provider's native response into an
	// OpenAI-compatible ChatResponse.
	TransformResponse(body []byte) (*ChatResponse, error)

	// TransformStreamEvent converts a single SSE data payload from the
	// provider's format into an OpenAI-compatible chunk JSON.
	// Returns the transformed bytes, whether the stream is done, and any error.
	TransformStreamEvent(data []byte) ([]byte, bool, error)
}

// GetTranslator returns the appropriate translator for a provider.
// OpenAI-compatible providers (openai, groq, together, mistral, deepseek, xai,
// fireworks, etc.) use the passthrough translator.
func GetTranslator(providerID string) ProviderTranslator {
	return GetProfile(providerID).Translator
}
