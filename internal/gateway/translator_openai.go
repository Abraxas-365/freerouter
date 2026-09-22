package gateway

import "encoding/json"

// OpenAITranslator is a passthrough — OpenAI-compatible providers need no
// request/response transformation.
type OpenAITranslator struct{}

func (t *OpenAITranslator) TransformRequest(body []byte, model string) ([]byte, error) {
	// Replace the model field with the provider's external model ID
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	raw["model"] = model
	return json.Marshal(raw)
}

func (t *OpenAITranslator) TransformResponse(body []byte) (*ChatResponse, error) {
	var resp ChatResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (t *OpenAITranslator) TransformStreamEvent(data []byte) ([]byte, bool, error) {
	s := string(data)
	if s == "[DONE]" {
		return data, true, nil
	}
	return data, false, nil
}
