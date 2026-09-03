package gateway

import "encoding/json"

// ============================================================================
// Embeddings Request/Response (OpenAI-compatible)
// ============================================================================

// EmbeddingRequest is the OpenAI-compatible embeddings request.
// See: https://platform.openai.com/docs/api-reference/embeddings/create
type EmbeddingRequest struct {
	Input          any    `json:"input"` // string or []string
	Model          string `json:"model"`
	EncodingFormat string `json:"encoding_format,omitempty"` // float, base64
	Dimensions     *int   `json:"dimensions,omitempty"`
	User           string `json:"user,omitempty"`
}

// EmbeddingResponse is the OpenAI-compatible embeddings response.
type EmbeddingResponse struct {
	Object string          `json:"object"` // "list"
	Data   []EmbeddingData `json:"data"`
	Model  string          `json:"model"`
	Usage  EmbeddingUsage  `json:"usage"`
}

// EmbeddingData holds a single embedding vector.
type EmbeddingData struct {
	Object    string `json:"object"` // "embedding"
	Embedding any    `json:"embedding"`
	Index     int    `json:"index"`
}

// EmbeddingUsage holds token usage for the embedding request.
type EmbeddingUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// ToJSON marshals the embedding response.
func (r *EmbeddingResponse) ToJSON() (json.RawMessage, error) {
	return json.Marshal(r)
}
