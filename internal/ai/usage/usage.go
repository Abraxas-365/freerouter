package usage

import (
	"net/http"
	"time"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/kernel"
)

// ============================================================================
// Usage Log Entity
// ============================================================================

// UsageLog records a single gateway request with token counts and costs
type UsageLog struct {
	ID        kernel.UsageLogID    `db:"id" json:"id"`
	TenantID  kernel.TenantID      `db:"tenant_id" json:"tenant_id"`
	APIKeyID  *kernel.APIKeyID        `db:"api_key_id" json:"api_key_id,omitempty"`
	KeyID     *kernel.ProviderKeyID `db:"provider_key_id" json:"provider_key_id,omitempty"`

	// Request info
	RequestedModel string `db:"requested_model" json:"requested_model"`
	UsedModel      string `db:"used_model" json:"used_model"`       // External model ID sent upstream
	UsedProvider   kernel.ProviderID `db:"used_provider" json:"used_provider"`
	MappingID      kernel.MappingID  `db:"mapping_id" json:"mapping_id"`

	// Tokens
	PromptTokens     int `db:"prompt_tokens" json:"prompt_tokens"`
	CompletionTokens int `db:"completion_tokens" json:"completion_tokens"`
	TotalTokens      int `db:"total_tokens" json:"total_tokens"`
	CachedTokens     int `db:"cached_tokens" json:"cached_tokens"`

	// Cost (USD)
	InputCost  float64 `db:"input_cost" json:"input_cost"`
	OutputCost float64 `db:"output_cost" json:"output_cost"`
	TotalCost  float64 `db:"total_cost" json:"total_cost"`

	// Request metadata
	DurationMs   int    `db:"duration_ms" json:"duration_ms"`
	Streamed     bool   `db:"streamed" json:"streamed"`
	StatusCode   int    `db:"status_code" json:"status_code"`
	FinishReason string `db:"finish_reason" json:"finish_reason"`
	HasError     bool   `db:"has_error" json:"has_error"`
	ErrorMessage string `db:"error_message" json:"error_message,omitempty"`

	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// ============================================================================
// DTOs
// ============================================================================

type UsageLogDTO struct {
	ID             kernel.UsageLogID `json:"id"`
	TenantID       kernel.TenantID  `json:"tenant_id"`
	RequestedModel string           `json:"requested_model"`
	UsedModel      string           `json:"used_model"`
	UsedProvider   kernel.ProviderID `json:"used_provider"`
	PromptTokens   int              `json:"prompt_tokens"`
	CompletionTokens int            `json:"completion_tokens"`
	TotalTokens    int              `json:"total_tokens"`
	InputCost      float64          `json:"input_cost"`
	OutputCost     float64          `json:"output_cost"`
	TotalCost      float64          `json:"total_cost"`
	DurationMs     int              `json:"duration_ms"`
	Streamed       bool             `json:"streamed"`
	StatusCode     int              `json:"status_code"`
	HasError       bool             `json:"has_error"`
	CreatedAt      time.Time        `json:"created_at"`
}

func (l *UsageLog) ToDTO() UsageLogDTO {
	return UsageLogDTO{
		ID:               l.ID,
		TenantID:         l.TenantID,
		RequestedModel:   l.RequestedModel,
		UsedModel:        l.UsedModel,
		UsedProvider:     l.UsedProvider,
		PromptTokens:     l.PromptTokens,
		CompletionTokens: l.CompletionTokens,
		TotalTokens:      l.TotalTokens,
		InputCost:        l.InputCost,
		OutputCost:       l.OutputCost,
		TotalCost:        l.TotalCost,
		DurationMs:       l.DurationMs,
		Streamed:         l.Streamed,
		StatusCode:       l.StatusCode,
		HasError:         l.HasError,
		CreatedAt:        l.CreatedAt,
	}
}

// ============================================================================
// Aggregation types
// ============================================================================

// UsageSummary aggregates usage over a time period
type UsageSummary struct {
	TenantID         kernel.TenantID `json:"tenant_id"`
	TotalRequests    int             `db:"total_requests" json:"total_requests"`
	TotalTokens      int             `db:"total_tokens" json:"total_tokens"`
	PromptTokens     int             `db:"prompt_tokens" json:"prompt_tokens"`
	CompletionTokens int             `db:"completion_tokens" json:"completion_tokens"`
	TotalCost        float64         `db:"total_cost" json:"total_cost"`
	ErrorCount       int             `db:"error_count" json:"error_count"`
}

// ModelUsageSummary aggregates usage per model
type ModelUsageSummary struct {
	Model            string  `db:"requested_model" json:"model"`
	TotalRequests    int     `db:"total_requests" json:"total_requests"`
	TotalTokens      int     `db:"total_tokens" json:"total_tokens"`
	PromptTokens     int     `db:"prompt_tokens" json:"prompt_tokens"`
	CompletionTokens int     `db:"completion_tokens" json:"completion_tokens"`
	TotalCost        float64 `db:"total_cost" json:"total_cost"`
}

// ============================================================================
// Query types
// ============================================================================

// UsageQuery for filtering usage logs
type UsageQuery struct {
	TenantID kernel.TenantID `json:"tenant_id"`
	Model    string          `json:"model,omitempty"`
	Provider string          `json:"provider,omitempty"`
	From     *time.Time      `json:"from,omitempty"`
	To       *time.Time      `json:"to,omitempty"`
	Limit    int             `json:"limit,omitempty"`
	Offset   int             `json:"offset,omitempty"`
}

// ============================================================================
// Response types
// ============================================================================

type UsageLogListResponse struct {
	Logs  []UsageLogDTO `json:"logs"`
	Total int           `json:"total"`
}

type UsageSummaryResponse struct {
	Summary        UsageSummary        `json:"summary"`
	ByModel        []ModelUsageSummary `json:"by_model"`
	PeriodStart    time.Time           `json:"period_start"`
	PeriodEnd      time.Time           `json:"period_end"`
}

// ============================================================================
// Errors
// ============================================================================

var ErrRegistry = errx.NewRegistry("USAGE")

var (
	CodeLogNotFound = ErrRegistry.Register("LOG_NOT_FOUND", errx.TypeNotFound, http.StatusNotFound, "Usage log not found")
)

func ErrLogNotFound() *errx.Error { return ErrRegistry.New(CodeLogNotFound) }
