package provider

import (
	"time"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/identity"
)

// ── ModelFallback ───────────────────────────────────────────────────

// ModelFallback defines a fallback relationship between two models.
// When the primary model fails, the system tries the fallback model.
type ModelFallback struct {
	ID              identity.ModelFallbackID `json:"id"               db:"id"`
	ModelID         identity.ModelID         `json:"model_id"         db:"model_id"`
	FallbackModelID identity.ModelID         `json:"fallback_model_id" db:"fallback_model_id"`
	Priority        int                      `json:"priority"         db:"priority"` // lower = higher priority
	Enabled         bool                     `json:"enabled"          db:"enabled"`
	CreatedAt       time.Time                `json:"created_at"       db:"created_at"`
}

// CreateFallback holds the data needed to define a model fallback.
type CreateFallback struct {
	ModelID         identity.ModelID `json:"model_id"`
	FallbackModelID identity.ModelID `json:"fallback_model_id"`
	Priority        int              `json:"priority"`
}

func (c CreateFallback) Validate() error {
	if c.ModelID.IsZero() {
		return errx.Validation("model_id is required")
	}
	if c.FallbackModelID.IsZero() {
		return errx.Validation("fallback_model_id is required")
	}
	if c.ModelID == c.FallbackModelID {
		return errx.Validation("model cannot be its own fallback")
	}
	return nil
}
