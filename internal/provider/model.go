package provider

import (
	"strings"
	"time"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/identity"
)

// ── Model Status ────────────────────────────────────────────────────

type ModelStatus string

const (
	ModelStatusActive   ModelStatus = "active"
	ModelStatusInactive ModelStatus = "inactive"
)

func (s ModelStatus) Valid() bool {
	return s == ModelStatusActive || s == ModelStatusInactive
}

// ── Model Stability ─────────────────────────────────────────────────

type ModelStability string

const (
	StabilityStable       ModelStability = "stable"
	StabilityBeta         ModelStability = "beta"
	StabilityExperimental ModelStability = "experimental"
)

func (s ModelStability) Valid() bool {
	return s == StabilityStable || s == StabilityBeta || s == StabilityExperimental
}

// ── Model ───────────────────────────────────────────────────────────

// Model represents an LLM model (gpt-4o, claude-sonnet, gemini-pro, etc.).
type Model struct {
	ID          identity.ModelID `json:"id"          db:"id"`
	Name        string           `json:"name"        db:"name"`
	Description string           `json:"description" db:"description"`
	Family      string           `json:"family"      db:"family"`
	Stability   ModelStability   `json:"stability"   db:"stability"`
	Status      ModelStatus      `json:"status"      db:"status"`
	Free        bool             `json:"free"        db:"free"`
	ReleasedAt  time.Time        `json:"released_at" db:"released_at"`
	CreatedAt   time.Time        `json:"created_at"  db:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"  db:"updated_at"`
}

// CreateModel holds the data needed to register a new model.
type CreateModel struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Family      string `json:"family"`
	Free        bool   `json:"free"`
}

func (c CreateModel) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return errx.Validation("model name is required")
	}
	if strings.TrimSpace(c.Family) == "" {
		return errx.Validation("model family is required")
	}
	return nil
}

// UpdateModel holds optional fields to patch an existing model.
type UpdateModel struct {
	Name        *string         `json:"name,omitempty"`
	Description *string         `json:"description,omitempty"`
	Family      *string         `json:"family,omitempty"`
	Stability   *ModelStability `json:"stability,omitempty"`
	Status      *ModelStatus    `json:"status,omitempty"`
	Free        *bool           `json:"free,omitempty"`
}

func (u UpdateModel) Validate() error {
	if u.Name != nil && strings.TrimSpace(*u.Name) == "" {
		return errx.Validation("model name cannot be empty")
	}
	if u.Stability != nil && !u.Stability.Valid() {
		return errx.Validation("invalid model stability")
	}
	if u.Status != nil && !u.Status.Valid() {
		return errx.Validation("invalid model status")
	}
	return nil
}

// ModelFilter holds criteria for listing models.
type ModelFilter struct {
	Status *ModelStatus // nil = all
	Family *string      // filter by model family
	Search *string      // partial match on name
}
