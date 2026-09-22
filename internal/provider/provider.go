package provider

import (
	"strings"
	"time"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/identity"
)

// ── Provider Status ─────────────────────────────────────────────────

// ProviderStatus represents the operational state of a provider.
type ProviderStatus string

const (
	ProviderStatusActive   ProviderStatus = "active"
	ProviderStatusInactive ProviderStatus = "inactive"
)

func (s ProviderStatus) Valid() bool {
	return s == ProviderStatusActive || s == ProviderStatusInactive
}

// ── Provider Protocol ──────────────────────────────────────────────

// Protocol identifies which wire protocol and auth style a provider uses.
type Protocol string

const (
	ProtocolOpenAI     Protocol = "openai"
	ProtocolAnthropic  Protocol = "anthropic"
	ProtocolGoogle     Protocol = "google"
	ProtocolAzure      Protocol = "azure"
	ProtocolCohere     Protocol = "cohere"
	ProtocolCodex      Protocol = "codex"
	ProtocolClaudeCode Protocol = "claude-code"
)

// ValidProtocols is the set of recognised protocols.
var ValidProtocols = []Protocol{
	ProtocolOpenAI, ProtocolAnthropic, ProtocolGoogle,
	ProtocolAzure, ProtocolCohere, ProtocolCodex, ProtocolClaudeCode,
}

func (p Protocol) Valid() bool {
	for _, v := range ValidProtocols {
		if p == v {
			return true
		}
	}
	return false
}

// ── Provider ────────────────────────────────────────────────────────

// Provider represents an LLM provider (OpenAI, Anthropic, Google, etc.).
type Provider struct {
	ID          identity.ProviderID `json:"id"          db:"id"`
	Name        string              `json:"name"        db:"name"`
	Protocol    Protocol            `json:"protocol"    db:"protocol"`
	Description string              `json:"description" db:"description"`
	Website     string              `json:"website,omitempty" db:"website"`
	BaseURL     string              `json:"base_url"    db:"base_url"`
	Status      ProviderStatus      `json:"status"      db:"status"`
	Streaming   bool                `json:"streaming"   db:"streaming"`
	CreatedAt   time.Time           `json:"created_at"  db:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"  db:"updated_at"`
}

// Create holds the data needed to register a new provider.
type Create struct {
	Name        string   `json:"name"`
	Protocol    Protocol `json:"protocol"`
	Description string   `json:"description"`
	Website     string   `json:"website"`
	BaseURL     string   `json:"base_url"`
	Streaming   bool     `json:"streaming"`
}

func (c Create) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return errx.Validation("provider name is required")
	}
	if strings.TrimSpace(c.BaseURL) == "" {
		return errx.Validation("provider base_url is required")
	}
	if !c.Protocol.Valid() {
		return errx.Validation("invalid protocol; must be one of: openai, anthropic, google, azure, cohere, codex, claude-code")
	}
	return nil
}

// Update holds optional fields to patch an existing provider.
type Update struct {
	Name        *string         `json:"name,omitempty"`
	Protocol    *Protocol       `json:"protocol,omitempty"`
	Description *string         `json:"description,omitempty"`
	Website     *string         `json:"website,omitempty"`
	BaseURL     *string         `json:"base_url,omitempty"`
	Status      *ProviderStatus `json:"status,omitempty"`
	Streaming   *bool           `json:"streaming,omitempty"`
}

func (u Update) Validate() error {
	if u.Name != nil && strings.TrimSpace(*u.Name) == "" {
		return errx.Validation("provider name cannot be empty")
	}
	if u.Status != nil && !u.Status.Valid() {
		return errx.Validation("invalid provider status")
	}
	if u.Protocol != nil && !u.Protocol.Valid() {
		return errx.Validation("invalid protocol; must be one of: openai, anthropic, google, azure, cohere, codex, claude-code")
	}
	return nil
}

// Filter holds domain-specific criteria for listing providers.
type Filter struct {
	Status *ProviderStatus // nil = all
	Search *string         // partial match on name
}
