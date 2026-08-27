package gateway

import "encoding/json"

// OpenAITranslator is a passthrough — the gateway's native format is already
// OpenAI-compatible, so no transformation is needed. This covers openai, groq,
// together, mistral, deepseek, xai, fireworks, perplexity, etc.
type OpenAITranslator struct{}

func (t *OpenAITranslator) TransformRequest(body []byte, model string) ([]byte, error) {
	return body, nil
}

func (t *OpenAITranslator) TransformResponse(body []byte) (*ChatResponse, error) {
	var resp ChatResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (t *OpenAITranslator) TransformStreamEvent(data []byte) ([]byte, bool, error) {
	if string(data) == "[DONE]" {
		return nil, true, nil
	}
	return data, false, nil
}
