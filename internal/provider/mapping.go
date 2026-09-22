package provider

import (
	"strings"
	"time"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/identity"
)

// ── ModelProviderMapping ────────────────────────────────────────────

// ModelProviderMapping links a model to a provider with pricing and capabilities.
type ModelProviderMapping struct {
	// Identity
	ID         identity.MappingID  `json:"id"          db:"id"`
	ModelID    identity.ModelID    `json:"model_id"    db:"model_id"`
	ProviderID identity.ProviderID `json:"provider_id" db:"provider_id"`
	ExternalID string              `json:"external_id" db:"external_id"`

	// Pricing (per million tokens)
	InputPrice       *float64 `json:"input_price,omitempty"        db:"input_price"`
	OutputPrice      *float64 `json:"output_price,omitempty"       db:"output_price"`
	CachedInputPrice *float64 `json:"cached_input_price,omitempty" db:"cached_input_price"`
	RequestPrice     *float64 `json:"request_price,omitempty"      db:"request_price"`
	ImageInputPrice  *float64 `json:"image_input_price,omitempty"  db:"image_input_price"`

	// Modality pricing
	AudioPricePerMinute   *float64 `json:"audio_price_per_minute,omitempty"    db:"audio_price_per_minute"`
	SpeechPricePer1kChars *float64 `json:"speech_price_per_1k_chars,omitempty" db:"speech_price_per_1k_chars"`
	RerankPricePer1k      *float64 `json:"rerank_price_per_1k,omitempty"       db:"rerank_price_per_1k"`

	// Limits
	ContextSize *int `json:"context_size,omitempty" db:"context_size"`
	MaxOutput   *int `json:"max_output,omitempty"   db:"max_output"`

	// Capabilities
	Streaming  bool `json:"streaming"   db:"streaming"`
	Vision     bool `json:"vision"      db:"vision"`
	Reasoning  bool `json:"reasoning"   db:"reasoning"`
	Tools      bool `json:"tools"       db:"tools"`
	JSONOutput bool `json:"json_output" db:"json_output"`
	Audio      bool `json:"audio"       db:"audio"`
	Speech     bool `json:"speech"      db:"speech"`
	Moderation bool `json:"moderation"  db:"moderation"`
	Rerank     bool `json:"rerank"      db:"rerank"`

	// Metadata
	Region    *string        `json:"region,omitempty" db:"region"`
	Stability ModelStability `json:"stability"        db:"stability"`
	Status    ModelStatus    `json:"status"           db:"status"`
	CreatedAt time.Time      `json:"created_at"       db:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"       db:"updated_at"`
}

// CreateMapping holds the data needed to link a model to a provider.
type CreateMapping struct {
	ModelID    identity.ModelID    `json:"model_id"`
	ProviderID identity.ProviderID `json:"provider_id"`
	ExternalID string              `json:"external_id"`

	// Pricing
	InputPrice       *float64 `json:"input_price,omitempty"`
	OutputPrice      *float64 `json:"output_price,omitempty"`
	CachedInputPrice *float64 `json:"cached_input_price,omitempty"`
	RequestPrice     *float64 `json:"request_price,omitempty"`
	ImageInputPrice  *float64 `json:"image_input_price,omitempty"`

	AudioPricePerMinute   *float64 `json:"audio_price_per_minute,omitempty"`
	SpeechPricePer1kChars *float64 `json:"speech_price_per_1k_chars,omitempty"`
	RerankPricePer1k      *float64 `json:"rerank_price_per_1k,omitempty"`

	// Limits
	ContextSize *int `json:"context_size,omitempty"`
	MaxOutput   *int `json:"max_output,omitempty"`

	// Capabilities
	Streaming  bool   `json:"streaming"`
	Vision     bool   `json:"vision"`
	Reasoning  bool   `json:"reasoning"`
	Tools      bool   `json:"tools"`
	JSONOutput bool   `json:"json_output"`
	Audio      bool   `json:"audio"`
	Speech     bool   `json:"speech"`
	Moderation bool   `json:"moderation"`
	Rerank     bool   `json:"rerank"`
	Region     string `json:"region,omitempty"`
}

func (c CreateMapping) Validate() error {
	if c.ModelID.IsZero() {
		return errx.Validation("model_id is required")
	}
	if c.ProviderID.IsZero() {
		return errx.Validation("provider_id is required")
	}
	if strings.TrimSpace(c.ExternalID) == "" {
		return errx.Validation("external_id is required")
	}
	return nil
}

// UpdateMapping holds optional fields to patch a mapping.
type UpdateMapping struct {
	ExternalID            *string      `json:"external_id,omitempty"`
	InputPrice            *float64     `json:"input_price,omitempty"`
	OutputPrice           *float64     `json:"output_price,omitempty"`
	CachedInputPrice      *float64     `json:"cached_input_price,omitempty"`
	RequestPrice          *float64     `json:"request_price,omitempty"`
	ImageInputPrice       *float64     `json:"image_input_price,omitempty"`
	AudioPricePerMinute   *float64     `json:"audio_price_per_minute,omitempty"`
	SpeechPricePer1kChars *float64     `json:"speech_price_per_1k_chars,omitempty"`
	RerankPricePer1k      *float64     `json:"rerank_price_per_1k,omitempty"`
	ContextSize           *int         `json:"context_size,omitempty"`
	MaxOutput             *int         `json:"max_output,omitempty"`
	Streaming             *bool        `json:"streaming,omitempty"`
	Vision                *bool        `json:"vision,omitempty"`
	Reasoning             *bool        `json:"reasoning,omitempty"`
	Tools                 *bool        `json:"tools,omitempty"`
	JSONOutput            *bool        `json:"json_output,omitempty"`
	Audio                 *bool        `json:"audio,omitempty"`
	Speech                *bool        `json:"speech,omitempty"`
	Moderation            *bool        `json:"moderation,omitempty"`
	Rerank                *bool        `json:"rerank,omitempty"`
	Region                *string      `json:"region,omitempty"`
	Status                *ModelStatus `json:"status,omitempty"`
}

func (u UpdateMapping) Validate() error {
	if u.ExternalID != nil && strings.TrimSpace(*u.ExternalID) == "" {
		return errx.Validation("external_id cannot be empty")
	}
	if u.Status != nil && !u.Status.Valid() {
		return errx.Validation("invalid mapping status")
	}
	return nil
}

// MappingFilter holds criteria for listing mappings.
type MappingFilter struct {
	ModelID    *identity.ModelID
	ProviderID *identity.ProviderID
	Status     *ModelStatus
}
