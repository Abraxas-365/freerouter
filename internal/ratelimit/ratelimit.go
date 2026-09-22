package ratelimit

import (
	"time"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/identity"
)

// ── Defaults ────────────────────────────────────────────────────────

const (
	DefaultRPM           = 60
	DefaultMaxConcurrent = 10
	DefaultSubjectID     = "default"
)

// ── RateLimitConfig ─────────────────────────────────────────────────

// RateLimitConfig holds rate limit settings. SubjectID can be "default"
// (applies to all subjects without a specific config) or a specific JWT
// subject identifier (user or service-account).
type RateLimitConfig struct {
	ID            identity.RateLimitConfigID `json:"id"             db:"id"`
	Name          string                     `json:"name"           db:"name"`
	SubjectID     string                     `json:"subject_id"     db:"subject_id"`
	RPM           int                        `json:"rpm"            db:"rpm"`
	MaxConcurrent int                        `json:"max_concurrent" db:"max_concurrent"`
	CreatedAt     time.Time                  `json:"created_at"     db:"created_at"`
	UpdatedAt     time.Time                  `json:"updated_at"     db:"updated_at"`
}

// ── Create ──────────────────────────────────────────────────────────

// CreateRateLimitConfig is the input for creating a rate limit config.
type CreateRateLimitConfig struct {
	Name          string `json:"name"`
	SubjectID     string `json:"subject_id"`
	RPM           int    `json:"rpm"`
	MaxConcurrent int    `json:"max_concurrent"`
}

// Validate checks that all required fields are present and valid.
func (c CreateRateLimitConfig) Validate() error {
	if c.Name == "" {
		return errx.Validation("name is required")
	}
	if c.SubjectID == "" {
		return errx.Validation("subject_id is required")
	}
	if c.RPM < 0 {
		return errx.Validation("rpm must be non-negative (0 = unlimited)")
	}
	if c.MaxConcurrent < 0 {
		return errx.Validation("max_concurrent must be non-negative (0 = unlimited)")
	}
	return nil
}

// ── Update ──────────────────────────────────────────────────────────

// UpdateRateLimitConfig holds optional fields for updating a config.
type UpdateRateLimitConfig struct {
	Name          *string `json:"name,omitempty"`
	RPM           *int    `json:"rpm,omitempty"`
	MaxConcurrent *int    `json:"max_concurrent,omitempty"`
}

// Validate checks update fields if provided.
func (u UpdateRateLimitConfig) Validate() error {
	if u.Name != nil && *u.Name == "" {
		return errx.Validation("name cannot be empty")
	}
	if u.RPM != nil && *u.RPM < 0 {
		return errx.Validation("rpm must be non-negative (0 = unlimited)")
	}
	if u.MaxConcurrent != nil && *u.MaxConcurrent < 0 {
		return errx.Validation("max_concurrent must be non-negative (0 = unlimited)")
	}
	return nil
}

// ── Filter ──────────────────────────────────────────────────────────

// Filter holds criteria for listing rate limit configs.
type Filter struct {
	SubjectID *string // exact match
	Name      *string // ILIKE match
}

// ── RateLimitResult ─────────────────────────────────────────────────

// RateLimitResult is returned by the limiter on every check.
type RateLimitResult struct {
	Allowed    bool          `json:"allowed"`
	Remaining  int           `json:"remaining"`
	Limit      int           `json:"limit"`
	RetryAfter time.Duration `json:"-"`
}
