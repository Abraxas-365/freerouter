// Package routingconfig manages per-subject routing strategy configuration
// for the gateway. It lets operators choose how the gateway orders candidate
// routes (cheapest, lowest-latency, round-robin) on a per-subject basis,
// falling back to a "default" subject config and finally to a hard-coded
// default strategy.
package routingconfig

import (
	"time"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/identity"
)

// ── Defaults ────────────────────────────────────────────────────────

const (
	// DefaultSubjectID is the fallback subject applied when no
	// subject-specific config exists.
	DefaultSubjectID = "default"

	// DefaultStrategy is used when no config exists for a subject and no
	// "default" config has been configured either.
	DefaultStrategy = StrategyCheapest
)

// Strategy determines the order in which candidate routes are tried.
// Mirrors gateway.RoutingStrategy so this package has no dependency on
// the gateway package (avoids an import cycle: gateway depends on this
// package's Queries interface).
type Strategy string

const (
	StrategyCheapest      Strategy = "cheapest"
	StrategyLowestLatency Strategy = "lowest-latency"
	StrategyRoundRobin    Strategy = "round-robin"
)

// ValidStrategies lists all accepted strategy values.
var ValidStrategies = []Strategy{StrategyCheapest, StrategyLowestLatency, StrategyRoundRobin}

// IsValid reports whether the strategy is a known value.
func (s Strategy) IsValid() bool {
	for _, v := range ValidStrategies {
		if s == v {
			return true
		}
	}
	return false
}

// ── RoutingConfig ───────────────────────────────────────────────────

// RoutingConfig holds the routing strategy for a subject. SubjectID can be
// "default" (applies to all subjects without a specific config) or a
// specific JWT subject identifier (user or service-account).
type RoutingConfig struct {
	ID        identity.RoutingConfigID `json:"id"         db:"id"`
	SubjectID string                   `json:"subject_id" db:"subject_id"`
	Strategy  Strategy                 `json:"strategy"   db:"strategy"`
	CreatedAt time.Time                `json:"created_at" db:"created_at"`
	UpdatedAt time.Time                `json:"updated_at" db:"updated_at"`
}

// ── Create ──────────────────────────────────────────────────────────

// CreateRoutingConfig is the input for creating a routing config.
type CreateRoutingConfig struct {
	SubjectID string   `json:"subject_id"`
	Strategy  Strategy `json:"strategy"`
}

// Validate checks that all required fields are present and valid.
func (c CreateRoutingConfig) Validate() error {
	if c.SubjectID == "" {
		return errx.Validation("subject_id is required")
	}
	if !c.Strategy.IsValid() {
		return errx.Validation("strategy must be one of: cheapest, lowest-latency, round-robin")
	}
	return nil
}

// ── Update ──────────────────────────────────────────────────────────

// UpdateRoutingConfig holds optional fields for updating a config.
type UpdateRoutingConfig struct {
	Strategy *Strategy `json:"strategy,omitempty"`
}

// Validate checks update fields if provided.
func (u UpdateRoutingConfig) Validate() error {
	if u.Strategy != nil && !u.Strategy.IsValid() {
		return errx.Validation("strategy must be one of: cheapest, lowest-latency, round-robin")
	}
	return nil
}

// ── Filter ──────────────────────────────────────────────────────────

// Filter holds criteria for listing routing configs.
type Filter struct {
	SubjectID *string // exact match
}
