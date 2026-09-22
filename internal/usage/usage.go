package usage

import (
	"encoding/json"
	"time"

	"github.com/Abraxas-365/freerouter/internal/identity"
)

// ── UsageLog ─────────────────────────────────────────────────────────

// UsageLog records a single gateway request with token counts, cost, and timing.
type UsageLog struct {
	ID    identity.UsageLogID    `json:"id"     db:"id"`
	KeyID identity.ProviderKeyID `json:"key_id" db:"key_id"`

	// Request routing
	RequestedModel string              `json:"requested_model" db:"requested_model"`
	UsedModel      string              `json:"used_model"      db:"used_model"`
	ProviderID     identity.ProviderID `json:"provider_id"     db:"provider_id"`
	MappingID      identity.MappingID  `json:"mapping_id"      db:"mapping_id"`

	// Tokens
	PromptTokens     int `json:"prompt_tokens"     db:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens" db:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"      db:"total_tokens"`
	CachedTokens     int `json:"cached_tokens"     db:"cached_tokens"`

	// Cost (USD)
	InputCost  float64 `json:"input_cost"  db:"input_cost"`
	OutputCost float64 `json:"output_cost" db:"output_cost"`
	TotalCost  float64 `json:"total_cost"  db:"total_cost"`

	// Request metadata
	DurationMs   int    `json:"duration_ms"   db:"duration_ms"`
	Streamed     bool   `json:"streamed"      db:"streamed"`
	StatusCode   int    `json:"status_code"   db:"status_code"`
	FinishReason string `json:"finish_reason" db:"finish_reason"`
	HasError     bool   `json:"has_error"     db:"has_error"`
	ErrorMessage string `json:"error_message,omitempty" db:"error_message"`
	IsFallback   bool   `json:"is_fallback"   db:"is_fallback"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// ── Filter ───────────────────────────────────────────────────────────

// Filter holds criteria for listing usage logs.
type Filter struct {
	Model    *string    // requested_model
	Provider *string    // provider ID
	HasError *bool      // only errors / only successes
	From     *time.Time // created_at >=
	To       *time.Time // created_at <=
}

// ── Summary ──────────────────────────────────────────────────────────

// Summary aggregates usage over a time period.
type Summary struct {
	TotalRequests    int     `json:"total_requests"    db:"total_requests"`
	TotalTokens      int     `json:"total_tokens"      db:"total_tokens"`
	PromptTokens     int     `json:"prompt_tokens"     db:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens" db:"completion_tokens"`
	TotalCost        float64 `json:"total_cost"        db:"total_cost"`
	ErrorCount       int     `json:"error_count"       db:"error_count"`
}

// ModelSummary aggregates usage per requested model.
type ModelSummary struct {
	Model            string  `json:"model"             db:"requested_model"`
	TotalRequests    int     `json:"total_requests"    db:"total_requests"`
	TotalTokens      int     `json:"total_tokens"      db:"total_tokens"`
	PromptTokens     int     `json:"prompt_tokens"     db:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens" db:"completion_tokens"`
	TotalCost        float64 `json:"total_cost"        db:"total_cost"`
}

// SummaryResponse combines aggregate + per-model breakdown for a period.
type SummaryResponse struct {
	Summary     Summary        `json:"summary"`
	ByModel     []ModelSummary `json:"by_model"`
	PeriodStart time.Time      `json:"period_start"`
	PeriodEnd   time.Time      `json:"period_end"`
}

// ── RequestContent ───────────────────────────────────────────────────

// RequestContent carries request/response content from handlers to the logger.
type RequestContent struct {
	Messages     json.RawMessage `json:"messages,omitempty"`
	ResponseBody json.RawMessage `json:"response_body,omitempty"`
}
