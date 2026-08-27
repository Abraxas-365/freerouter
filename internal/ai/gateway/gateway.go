package gateway

import "time"

// ============================================================================
// Chat Completion Request (OpenAI-compatible)
// ============================================================================

// ChatRequest is the OpenAI-compatible chat completion request
type ChatRequest struct {
	Model            string          `json:"model"`
	Messages         []Message       `json:"messages"`
	Temperature      *float64        `json:"temperature,omitempty"`
	MaxTokens        *int            `json:"max_tokens,omitempty"`
	TopP             *float64        `json:"top_p,omitempty"`
	FrequencyPenalty *float64        `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64        `json:"presence_penalty,omitempty"`
	Stream           bool            `json:"stream"`
	N                *int            `json:"n,omitempty"`
	Stop             any             `json:"stop,omitempty"` // string or []string
	User             string          `json:"user,omitempty"`
	ResponseFormat   *ResponseFormat `json:"response_format,omitempty"`
	Tools            []Tool          `json:"tools,omitempty"`
	ToolChoice       any             `json:"tool_choice,omitempty"` // "auto"|"none"|"required"|object
	ReasoningEffort  string          `json:"reasoning_effort,omitempty"`
}

// Message represents a chat message
type Message struct {
	Role       string     `json:"role"` // system, user, assistant, tool
	Content    any        `json:"content"`
	Name       string     `json:"name,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ResponseFormat specifies the response format
type ResponseFormat struct {
	Type       string `json:"type"` // text, json_object, json_schema
	JSONSchema any    `json:"json_schema,omitempty"`
}

// Tool describes a tool available to the model
type Tool struct {
	Type     string       `json:"type"` // function
	Function ToolFunction `json:"function"`
}

// ToolFunction describes a function tool
type ToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

// ToolCall represents a tool call made by the model
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"` // function
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction represents the function details in a tool call
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ============================================================================
// Chat Completion Response (OpenAI-compatible)
// ============================================================================

// ChatResponse is the non-streaming response
type ChatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"` // "chat.completion"
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   *Usage   `json:"usage,omitempty"`
}

// Choice represents a response choice
type Choice struct {
	Index        int      `json:"index"`
	Message      *Message `json:"message,omitempty"`       // Non-streaming
	Delta        *Message `json:"delta,omitempty"`          // Streaming
	FinishReason *string  `json:"finish_reason,omitempty"` // stop, tool_calls, length, content_filter
}

// Usage tracks token usage
type Usage struct {
	PromptTokens            int `json:"prompt_tokens"`
	CompletionTokens        int `json:"completion_tokens"`
	TotalTokens             int `json:"total_tokens"`
	CacheReadInputTokens    int `json:"cache_read_input_tokens,omitempty"`
	CacheCreationInputToken int `json:"cache_creation_input_tokens,omitempty"`
}

// ChatStreamChunk is a single SSE chunk in a streaming response
type ChatStreamChunk struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"` // "chat.completion.chunk"
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   *Usage   `json:"usage,omitempty"`
}

// ============================================================================
// Routing context (internal, not exposed to clients)
// ============================================================================

// RouteResult holds the resolved provider context for a request
type RouteResult struct {
	ProviderID  string // e.g. "openai"
	ExternalID  string // Provider's model identifier (e.g. "gpt-4o-2024-08-06")
	MappingID   string // model_provider_mapping ID
	Token       string // Decrypted provider API key
	BaseURL     string // Upstream endpoint (e.g. "https://api.openai.com/v1")
	KeyID       string // provider_key ID (for logging)
	InputPrice  *float64
	OutputPrice *float64
}

// RequestLog captures metadata about a completed gateway request
type RequestLog struct {
	ID               string        `json:"id"`
	Model            string        `json:"model"`
	ProviderID       string        `json:"provider_id"`
	ExternalModel    string        `json:"external_model"`
	Status           int           `json:"status"`
	PromptTokens     int           `json:"prompt_tokens"`
	CompletionTokens int           `json:"completion_tokens"`
	TotalTokens      int           `json:"total_tokens"`
	InputCost        float64       `json:"input_cost"`
	OutputCost       float64       `json:"output_cost"`
	TotalCost        float64       `json:"total_cost"`
	Duration         time.Duration `json:"duration_ms"`
	Streamed         bool          `json:"streamed"`
	Error            string        `json:"error,omitempty"`
}
