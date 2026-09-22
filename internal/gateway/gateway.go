package gateway

import (
	"time"

	"github.com/Abraxas-365/freerouter/internal/identity"
)

// ============================================================================
// Chat Completion Request (OpenAI-compatible)
// ============================================================================

// ChatRequest is the OpenAI-compatible chat completion request.
type ChatRequest struct {
	Model               string          `json:"model"`
	Messages            []Message       `json:"messages"`
	Temperature         *float64        `json:"temperature,omitempty"`
	MaxTokens           *int            `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int            `json:"max_completion_tokens,omitempty"`
	TopP                *float64        `json:"top_p,omitempty"`
	FrequencyPenalty    *float64        `json:"frequency_penalty,omitempty"`
	PresencePenalty     *float64        `json:"presence_penalty,omitempty"`
	Stream              bool            `json:"stream"`
	StreamOptions       *StreamOptions  `json:"stream_options,omitempty"`
	N                   *int            `json:"n,omitempty"`
	Stop                any             `json:"stop,omitempty"`
	User                string          `json:"user,omitempty"`
	ResponseFormat      *ResponseFormat `json:"response_format,omitempty"`
	Tools               []Tool          `json:"tools,omitempty"`
	ToolChoice          any             `json:"tool_choice,omitempty"`
	ReasoningEffort     string          `json:"reasoning_effort,omitempty"`
	Thinking            *ThinkingConfig `json:"thinking,omitempty"`
	OutputConfig        *OutputConfig   `json:"output_config,omitempty"`
	Seed                *int            `json:"seed,omitempty"`
	TopK                *int            `json:"top_k,omitempty"`
	LogProbs            *bool           `json:"logprobs,omitempty"`
	TopLogProbs         *int            `json:"top_logprobs,omitempty"`
	ServiceTier         string          `json:"service_tier,omitempty"`
}

// StreamOptions controls streaming behavior.
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

// ThinkingConfig controls Anthropic extended thinking. Passed through exactly
// on the /v1/messages path; on /v1/chat/completions the ReasoningEffort field
// is mapped to this instead.
type ThinkingConfig struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens,omitempty"`
	Display      string `json:"display,omitempty"` // "summarized", "omitted", or empty
}

// OutputConfig controls Anthropic output effort (adaptive thinking depth).
type OutputConfig struct {
	Effort string `json:"effort,omitempty"` // "low", "medium", "high", "max", "xhigh"
}

// Message represents a chat message.
type Message struct {
	Role         string                 `json:"role"`
	Content      any                    `json:"content"`
	Name         string                 `json:"name,omitempty"`
	Reasoning    string                 `json:"reasoning,omitempty"`
	Signature    string                 `json:"signature,omitempty"`
	ToolCalls    []ToolCall             `json:"tool_calls,omitempty"`
	ToolCallID   string                 `json:"tool_call_id,omitempty"`
	CacheControl map[string]interface{} `json:"cache_control,omitempty"`
}

// ResponseFormat specifies the response format.
type ResponseFormat struct {
	Type       string `json:"type"`
	JSONSchema any    `json:"json_schema,omitempty"`
}

// Tool describes a tool available to the model.
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction describes a function tool.
type ToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

// ToolCall represents a tool call made by the model.
type ToolCall struct {
	Index    int              `json:"index"`
	ID       string           `json:"id,omitempty"`
	Type     string           `json:"type,omitempty"`
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction represents the function details in a tool call.
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ============================================================================
// Chat Completion Response (OpenAI-compatible)
// ============================================================================

// ChatResponse is the non-streaming response.
type ChatResponse struct {
	ID          string   `json:"id"`
	Object      string   `json:"object"`
	Created     int64    `json:"created"`
	Model       string   `json:"model"`
	Choices     []Choice `json:"choices"`
	Usage       *Usage   `json:"usage,omitempty"`
	ServiceTier string   `json:"service_tier,omitempty"`
}

// Choice represents a response choice.
type Choice struct {
	Index        int      `json:"index"`
	Message      *Message `json:"message,omitempty"`
	Delta        *Message `json:"delta,omitempty"`
	FinishReason *string  `json:"finish_reason,omitempty"`
	LogProbs     any      `json:"logprobs,omitempty"`
}

// Usage tracks token counts.
type Usage struct {
	PromptTokens            int                     `json:"prompt_tokens"`
	CompletionTokens        int                     `json:"completion_tokens"`
	TotalTokens             int                     `json:"total_tokens"`
	CacheReadInputTokens    int                     `json:"cache_read_input_tokens,omitempty"`
	CacheCreationInputToken int                     `json:"cache_creation_input_tokens,omitempty"`
	CompletionTokensDetails *CompletionTokenDetails `json:"completion_tokens_details,omitempty"`
}

// CompletionTokenDetails provides a breakdown of completion token usage.
type CompletionTokenDetails struct {
	ReasoningTokens int `json:"reasoning_tokens,omitempty"`
}

// ChatStreamChunk is a single SSE chunk in the streaming response.
type ChatStreamChunk struct {
	ID          string   `json:"id,omitempty"`
	Object      string   `json:"object"`
	Created     int64    `json:"created"`
	Model       string   `json:"model,omitempty"`
	Choices     []Choice `json:"choices"`
	Usage       *Usage   `json:"usage,omitempty"`
	ServiceTier string   `json:"service_tier,omitempty"`
}

// ============================================================================
// Route Result
// ============================================================================

// RouteResult is the output of routing: a resolved provider + key + mapping.
type RouteResult struct {
	ProviderID identity.ProviderID    `json:"provider_id"`
	Protocol   string                 `json:"protocol"` // provider.Protocol — used for profile lookup
	KeyID      identity.ProviderKeyID `json:"key_id"`
	KeyType    string                 `json:"-"` // "api_key" or "oauth"
	MappingID  identity.MappingID     `json:"mapping_id"`
	ModelID    identity.ModelID       `json:"model_id"`
	ExternalID string                 `json:"external_id"`
	Token      string                 `json:"-"`
	BaseURL    string                 `json:"base_url"`
	IsFallback bool                   `json:"is_fallback"`

	// Pricing (per million tokens)
	InputPrice       *float64 `json:"-"`
	OutputPrice      *float64 `json:"-"`
	CachedInputPrice *float64 `json:"-"`

	// Modality pricing
	AudioPricePerMinute   *float64 `json:"-"`
	SpeechPricePer1kChars *float64 `json:"-"`
	RerankPricePer1k      *float64 `json:"-"`
}

// ProfileSlug returns the profile registry key for this route.
// Uses the provider's protocol field, which maps directly to profile keys.
func (r *RouteResult) ProfileSlug() string {
	return r.Protocol
}

// ============================================================================
// Token estimation
// ============================================================================

// EstimateTokens approximates token count from text (~4 chars per token).
func EstimateTokens(text string) int {
	if len(text) == 0 {
		return 0
	}
	tokens := len(text) / 4
	if tokens == 0 {
		tokens = 1
	}
	return tokens
}

// EstimateMessageTokens estimates the total input tokens for a set of messages.
func EstimateMessageTokens(messages []Message) int {
	total := 0
	for _, m := range messages {
		total += 4 // per-message overhead
		switch v := m.Content.(type) {
		case string:
			total += EstimateTokens(v)
		}
		if m.Name != "" {
			total += EstimateTokens(m.Name)
		}
	}
	total += 2 // reply priming
	return total
}

// ============================================================================
// Cost calculation (chat)
// ============================================================================

// CalculateChatCost computes the cost for a chat request in USD.
func CalculateChatCost(route *RouteResult, promptTokens, completionTokens int) float64 {
	cost := 0.0
	if route.InputPrice != nil {
		cost += float64(promptTokens) * *route.InputPrice / 1_000_000
	}
	if route.OutputPrice != nil {
		cost += float64(completionTokens) * *route.OutputPrice / 1_000_000
	}
	return cost
}

// ============================================================================
// OpenAI-compatible model list
// ============================================================================

// ModelObject is the OpenAI-compatible model descriptor for GET /v1/models.
type ModelObject struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// ModelList is the OpenAI-compatible model list response.
type ModelList struct {
	Object string        `json:"object"`
	Data   []ModelObject `json:"data"`
}

// NewModelObject creates a model object from name and creation time.
func NewModelObject(name, family string, created time.Time) ModelObject {
	return ModelObject{
		ID:      name,
		Object:  "model",
		Created: created.Unix(),
		OwnedBy: family,
	}
}

// ============================================================================
// Cost estimation (pre-flight, informational only — no billing)
// ============================================================================

// CostEstimateRequest is the input for pre-flight cost estimation.
type CostEstimateRequest struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	MaxTokens *int      `json:"max_tokens,omitempty"`
}

// CostEstimateResponse is the pre-flight cost estimate for a chat request.
type CostEstimateResponse struct {
	Model                 string  `json:"model"`
	EstimatedInputTokens  int     `json:"estimated_input_tokens"`
	EstimatedOutputTokens int     `json:"estimated_output_tokens"`
	InputCostUSD          float64 `json:"input_cost_usd"`
	OutputCostUSD         float64 `json:"output_cost_usd"`
	TotalCostUSD          float64 `json:"total_cost_usd"`
}

// DefaultEstimateMaxOutputTokens is the assumed output length used for cost
// estimation when the caller does not specify max_tokens.
const DefaultEstimateMaxOutputTokens = 4096
