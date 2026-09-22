package usage

import (
	"time"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/identity"
)

// ── Defaults ────────────────────────────────────────────────────────

const DefaultRetentionDays = 90

// ── RetentionConfig ─────────────────────────────────────────────────

// RetentionConfig is the single global usage-log data retention configuration:
// how long logs are kept, and whether message/response-body content is
// retained at all. There is exactly one config for the whole gateway (no
// per-tenant scoping in v2 — IAMKit handles multi-tenancy upstream).
type RetentionConfig struct {
	ID                 identity.RetentionConfigID `json:"id"                   db:"id"`
	RetentionDays      int                        `json:"retention_days"       db:"retention_days"`
	RetainMessages     bool                       `json:"retain_messages"      db:"retain_messages"`
	RetainResponseBody bool                       `json:"retain_response_body" db:"retain_response_body"`
	CreatedAt          time.Time                  `json:"created_at"           db:"created_at"`
	UpdatedAt          time.Time                  `json:"updated_at"           db:"updated_at"`
}

// DefaultRetentionConfig returns the built-in default retention settings.
func DefaultRetentionConfig() RetentionConfig {
	return RetentionConfig{
		RetentionDays:      DefaultRetentionDays,
		RetainMessages:     true,
		RetainResponseBody: true,
	}
}

// ── Upsert ──────────────────────────────────────────────────────────

// UpsertRetention is the input for creating/updating the global retention config.
type UpsertRetention struct {
	RetentionDays      *int  `json:"retention_days,omitempty"`
	RetainMessages     *bool `json:"retain_messages,omitempty"`
	RetainResponseBody *bool `json:"retain_response_body,omitempty"`
}

// Validate checks that provided fields are valid. RetentionDays must be
// non-negative; 0 means "retain forever" (purge worker no-ops).
func (u UpsertRetention) Validate() error {
	if u.RetentionDays != nil && *u.RetentionDays < 0 {
		return errx.Validation("retention_days must be non-negative (0 = retain forever)")
	}
	return nil
}
