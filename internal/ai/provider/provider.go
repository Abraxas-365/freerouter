package provider

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/kernel"
)

// ============================================================================
// Provider Entity
// ============================================================================

// ProviderStatus defines the status of a provider
type ProviderStatus string

const (
	ProviderStatusActive   ProviderStatus = "active"
	ProviderStatusInactive ProviderStatus = "inactive"
)

// Provider represents an LLM provider (e.g. OpenAI, Anthropic, Google)
type Provider struct {
	ID          kernel.ProviderID `db:"id" json:"id"`
	Name        string         `db:"name" json:"name"`
	Description string         `db:"description" json:"description"`
	Website     string         `db:"website" json:"website,omitempty"`
	Status      ProviderStatus `db:"status" json:"status"`
	Streaming   bool           `db:"streaming" json:"streaming"`
	CreatedAt   time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time      `db:"updated_at" json:"updated_at"`
}

func (p *Provider) IsActive() bool {
	return p.Status == ProviderStatusActive
}

func (p *Provider) Activate() {
	p.Status = ProviderStatusActive
	p.UpdatedAt = time.Now().UTC()
}

func (p *Provider) Deactivate() {
	p.Status = ProviderStatusInactive
	p.UpdatedAt = time.Now().UTC()
}

// ============================================================================
// Model Entity
// ============================================================================

// ModelStatus defines the status of a model
type ModelStatus string

const (
	ModelStatusActive   ModelStatus = "active"
	ModelStatusInactive ModelStatus = "inactive"
)

// ModelStability defines the stability level of a model
type ModelStability string

const (
	StabilityStable       ModelStability = "stable"
	StabilityBeta         ModelStability = "beta"
	StabilityExperimental ModelStability = "experimental"
)

// Model represents an LLM model (e.g. gpt-4o, claude-sonnet-4-20250514)
type Model struct {
	ID          kernel.ModelID `db:"id" json:"id"`
	Name        string         `db:"name" json:"name"`
	Description string         `db:"description" json:"description"`
	Family      string         `db:"family" json:"family"`
	Stability   ModelStability `db:"stability" json:"stability"`
	Status      ModelStatus    `db:"status" json:"status"`
	Free        bool           `db:"free" json:"free"`
	ReleasedAt  time.Time      `db:"released_at" json:"released_at"`
	CreatedAt   time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time      `db:"updated_at" json:"updated_at"`
}

func (m *Model) IsActive() bool {
	return m.Status == ModelStatusActive
}

// ============================================================================
// Model-Provider Mapping Entity
// ============================================================================

// ModelProviderMapping links a model to a provider with pricing and capabilities
type ModelProviderMapping struct {
	ID         kernel.MappingID  `db:"id" json:"id"`
	ModelID    kernel.ModelID    `db:"model_id" json:"model_id"`
	ProviderID kernel.ProviderID `db:"provider_id" json:"provider_id"`
	ExternalID string `db:"external_id" json:"external_id"` // Provider's model identifier

	// Pricing (per million tokens)
	InputPrice       *float64 `db:"input_price" json:"input_price,omitempty"`
	OutputPrice      *float64 `db:"output_price" json:"output_price,omitempty"`
	CachedInputPrice *float64 `db:"cached_input_price" json:"cached_input_price,omitempty"`
	RequestPrice     *float64 `db:"request_price" json:"request_price,omitempty"`
	ImageInputPrice  *float64 `db:"image_input_price" json:"image_input_price,omitempty"`

	// Limits
	ContextSize *int `db:"context_size" json:"context_size,omitempty"`
	MaxOutput   *int `db:"max_output" json:"max_output,omitempty"`

	// Capabilities
	Streaming  bool `db:"streaming" json:"streaming"`
	Vision     bool `db:"vision" json:"vision"`
	Reasoning  bool `db:"reasoning" json:"reasoning"`
	Tools      bool `db:"tools" json:"tools"`
	JSONOutput bool `db:"json_output" json:"json_output"`

	Region    *string        `db:"region" json:"region,omitempty"`
	Stability ModelStability `db:"stability" json:"stability"`
	Status    ModelStatus    `db:"status" json:"status"`
	CreatedAt time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt time.Time      `db:"updated_at" json:"updated_at"`
}

func (m *ModelProviderMapping) IsActive() bool {
	return m.Status == ModelStatusActive
}

// ============================================================================
// DTOs
// ============================================================================

// ProviderDTO is the external representation of a provider
type ProviderDTO struct {
	ID          kernel.ProviderID `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Website     string         `json:"website,omitempty"`
	Status      ProviderStatus `json:"status"`
	Streaming   bool           `json:"streaming"`
	CreatedAt   time.Time      `json:"created_at"`
}

func (p *Provider) ToDTO() ProviderDTO {
	return ProviderDTO{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		Website:     p.Website,
		Status:      p.Status,
		Streaming:   p.Streaming,
		CreatedAt:   p.CreatedAt,
	}
}

// ModelDTO is the external representation of a model
type ModelDTO struct {
	ID          kernel.ModelID `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Family      string         `json:"family"`
	Stability   ModelStability `json:"stability"`
	Status      ModelStatus    `json:"status"`
	Free        bool           `json:"free"`
	ReleasedAt  time.Time      `json:"released_at"`
	CreatedAt   time.Time      `json:"created_at"`
}

func (m *Model) ToDTO() ModelDTO {
	return ModelDTO{
		ID:          m.ID,
		Name:        m.Name,
		Description: m.Description,
		Family:      m.Family,
		Stability:   m.Stability,
		Status:      m.Status,
		Free:        m.Free,
		ReleasedAt:  m.ReleasedAt,
		CreatedAt:   m.CreatedAt,
	}
}

// ModelProviderMappingDTO is the external representation of a mapping
type ModelProviderMappingDTO struct {
	ID         kernel.MappingID  `json:"id"`
	ModelID    kernel.ModelID    `json:"model_id"`
	ProviderID kernel.ProviderID `json:"provider_id"`
	ExternalID string `json:"external_id"`

	InputPrice       *float64 `json:"input_price,omitempty"`
	OutputPrice      *float64 `json:"output_price,omitempty"`
	CachedInputPrice *float64 `json:"cached_input_price,omitempty"`
	RequestPrice     *float64 `json:"request_price,omitempty"`
	ImageInputPrice  *float64 `json:"image_input_price,omitempty"`

	ContextSize *int `json:"context_size,omitempty"`
	MaxOutput   *int `json:"max_output,omitempty"`

	Streaming  bool `json:"streaming"`
	Vision     bool `json:"vision"`
	Reasoning  bool `json:"reasoning"`
	Tools      bool `json:"tools"`
	JSONOutput bool `json:"json_output"`

	Region    *string        `json:"region,omitempty"`
	Stability ModelStability `json:"stability"`
	Status    ModelStatus    `json:"status"`
}

func (m *ModelProviderMapping) ToDTO() ModelProviderMappingDTO {
	return ModelProviderMappingDTO{
		ID:               m.ID,
		ModelID:          m.ModelID,
		ProviderID:       m.ProviderID,
		ExternalID:       m.ExternalID,
		InputPrice:       m.InputPrice,
		OutputPrice:      m.OutputPrice,
		CachedInputPrice: m.CachedInputPrice,
		RequestPrice:     m.RequestPrice,
		ImageInputPrice:  m.ImageInputPrice,
		ContextSize:      m.ContextSize,
		MaxOutput:        m.MaxOutput,
		Streaming:        m.Streaming,
		Vision:           m.Vision,
		Reasoning:        m.Reasoning,
		Tools:            m.Tools,
		JSONOutput:       m.JSONOutput,
		Region:           m.Region,
		Stability:        m.Stability,
		Status:           m.Status,
	}
}

// ModelWithMappings combines a model with all its provider mappings
type ModelWithMappings struct {
	Model    ModelDTO                  `json:"model"`
	Mappings []ModelProviderMappingDTO `json:"mappings"`
}

// ============================================================================
// Request types
// ============================================================================

// CreateProviderRequest for creating a new provider
type CreateProviderRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Website     string `json:"website"`
	Streaming   bool   `json:"streaming"`
}

func (r *CreateProviderRequest) Validate() error {
	r.Name = strings.TrimSpace(r.Name)
	if utf8.RuneCountInString(r.Name) < 2 {
		return errx.Validation("Provider name is required and must be at least 2 characters").
			WithDetail("field", "name")
	}
	return nil
}

// UpdateProviderRequest for updating a provider
type UpdateProviderRequest struct {
	Name        *string         `json:"name,omitempty"`
	Description *string         `json:"description,omitempty"`
	Website     *string         `json:"website,omitempty"`
	Status      *ProviderStatus `json:"status,omitempty"`
	Streaming   *bool           `json:"streaming,omitempty"`
}

func (r *UpdateProviderRequest) Validate() error {
	if r.Name != nil && utf8.RuneCountInString(strings.TrimSpace(*r.Name)) < 2 {
		return errx.Validation("Provider name must be at least 2 characters").
			WithDetail("field", "name")
	}
	if r.Status != nil {
		switch *r.Status {
		case ProviderStatusActive, ProviderStatusInactive:
		default:
			return errx.Validation("Invalid provider status").WithDetail("field", "status")
		}
	}
	return nil
}

// CreateModelRequest for creating a new model
type CreateModelRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Family      string `json:"family"`
	Free        bool   `json:"free"`
}

func (r *CreateModelRequest) Validate() error {
	r.Name = strings.TrimSpace(r.Name)
	if utf8.RuneCountInString(r.Name) < 2 {
		return errx.Validation("Model name is required and must be at least 2 characters").
			WithDetail("field", "name")
	}
	r.Family = strings.TrimSpace(r.Family)
	if r.Family == "" {
		return errx.Validation("Model family is required").WithDetail("field", "family")
	}
	return nil
}

// UpdateModelRequest for updating a model
type UpdateModelRequest struct {
	Name        *string        `json:"name,omitempty"`
	Description *string        `json:"description,omitempty"`
	Family      *string        `json:"family,omitempty"`
	Stability   *ModelStability `json:"stability,omitempty"`
	Status      *ModelStatus   `json:"status,omitempty"`
	Free        *bool          `json:"free,omitempty"`
}

func (r *UpdateModelRequest) Validate() error {
	if r.Name != nil && utf8.RuneCountInString(strings.TrimSpace(*r.Name)) < 2 {
		return errx.Validation("Model name must be at least 2 characters").
			WithDetail("field", "name")
	}
	if r.Stability != nil {
		switch *r.Stability {
		case StabilityStable, StabilityBeta, StabilityExperimental:
		default:
			return errx.Validation("Invalid model stability").WithDetail("field", "stability")
		}
	}
	if r.Status != nil {
		switch *r.Status {
		case ModelStatusActive, ModelStatusInactive:
		default:
			return errx.Validation("Invalid model status").WithDetail("field", "status")
		}
	}
	return nil
}

// CreateMappingRequest for linking a model to a provider
type CreateMappingRequest struct {
	ModelID    kernel.ModelID    `json:"model_id"`
	ProviderID kernel.ProviderID `json:"provider_id"`
	ExternalID string `json:"external_id"`

	InputPrice       *float64 `json:"input_price,omitempty"`
	OutputPrice      *float64 `json:"output_price,omitempty"`
	CachedInputPrice *float64 `json:"cached_input_price,omitempty"`
	RequestPrice     *float64 `json:"request_price,omitempty"`
	ImageInputPrice  *float64 `json:"image_input_price,omitempty"`

	ContextSize *int `json:"context_size,omitempty"`
	MaxOutput   *int `json:"max_output,omitempty"`

	Streaming  bool   `json:"streaming"`
	Vision     bool   `json:"vision"`
	Reasoning  bool   `json:"reasoning"`
	Tools      bool   `json:"tools"`
	JSONOutput bool   `json:"json_output"`
	Region     string `json:"region,omitempty"`
}

func (r *CreateMappingRequest) Validate() error {
	if r.ModelID.IsEmpty() {
		return errx.Validation("Model ID is required").WithDetail("field", "model_id")
	}
	if r.ProviderID.IsEmpty() {
		return errx.Validation("Provider ID is required").WithDetail("field", "provider_id")
	}
	if strings.TrimSpace(r.ExternalID) == "" {
		return errx.Validation("External ID is required").WithDetail("field", "external_id")
	}
	return nil
}

// UpdateMappingRequest for updating a model-provider mapping
type UpdateMappingRequest struct {
	ExternalID       *string  `json:"external_id,omitempty"`
	InputPrice       *float64 `json:"input_price,omitempty"`
	OutputPrice      *float64 `json:"output_price,omitempty"`
	CachedInputPrice *float64 `json:"cached_input_price,omitempty"`
	RequestPrice     *float64 `json:"request_price,omitempty"`
	ImageInputPrice  *float64 `json:"image_input_price,omitempty"`
	ContextSize      *int     `json:"context_size,omitempty"`
	MaxOutput        *int     `json:"max_output,omitempty"`
	Streaming        *bool    `json:"streaming,omitempty"`
	Vision           *bool    `json:"vision,omitempty"`
	Reasoning        *bool    `json:"reasoning,omitempty"`
	Tools            *bool    `json:"tools,omitempty"`
	JSONOutput       *bool    `json:"json_output,omitempty"`
	Region           *string  `json:"region,omitempty"`
	Status           *ModelStatus `json:"status,omitempty"`
}

func (r *UpdateMappingRequest) Validate() error {
	if r.Status != nil {
		switch *r.Status {
		case ModelStatusActive, ModelStatusInactive:
		default:
			return errx.Validation("Invalid mapping status").WithDetail("field", "status")
		}
	}
	return nil
}

// ============================================================================
// Response types
// ============================================================================

// ProviderListResponse for listing providers
type ProviderListResponse struct {
	Providers []ProviderDTO `json:"providers"`
	Total     int           `json:"total"`
}

// ModelListResponse for listing models
type ModelListResponse struct {
	Models []ModelDTO `json:"models"`
	Total  int        `json:"total"`
}

// MappingListResponse for listing mappings
type MappingListResponse struct {
	Mappings []ModelProviderMappingDTO `json:"mappings"`
	Total    int                       `json:"total"`
}

// ============================================================================
// Errors
// ============================================================================

var ErrRegistry = errx.NewRegistry("PROVIDER")

var (
	CodeProviderNotFound      = ErrRegistry.Register("PROVIDER_NOT_FOUND", errx.TypeNotFound, http.StatusNotFound, "Provider not found")
	CodeProviderAlreadyExists = ErrRegistry.Register("PROVIDER_ALREADY_EXISTS", errx.TypeConflict, http.StatusConflict, "Provider already exists")
	CodeModelNotFound         = ErrRegistry.Register("MODEL_NOT_FOUND", errx.TypeNotFound, http.StatusNotFound, "Model not found")
	CodeModelAlreadyExists    = ErrRegistry.Register("MODEL_ALREADY_EXISTS", errx.TypeConflict, http.StatusConflict, "Model already exists")
	CodeMappingNotFound       = ErrRegistry.Register("MAPPING_NOT_FOUND", errx.TypeNotFound, http.StatusNotFound, "Model-provider mapping not found")
	CodeMappingAlreadyExists  = ErrRegistry.Register("MAPPING_ALREADY_EXISTS", errx.TypeConflict, http.StatusConflict, "Model-provider mapping already exists")
)

func ErrProviderNotFound() *errx.Error      { return ErrRegistry.New(CodeProviderNotFound) }
func ErrProviderAlreadyExists() *errx.Error  { return ErrRegistry.New(CodeProviderAlreadyExists) }
func ErrModelNotFound() *errx.Error          { return ErrRegistry.New(CodeModelNotFound) }
func ErrModelAlreadyExists() *errx.Error     { return ErrRegistry.New(CodeModelAlreadyExists) }
func ErrMappingNotFound() *errx.Error        { return ErrRegistry.New(CodeMappingNotFound) }
func ErrMappingAlreadyExists() *errx.Error   { return ErrRegistry.New(CodeMappingAlreadyExists) }

// ============================================================================
// Model Fallback Entity
// ============================================================================

// ModelFallback defines a fallback relationship between two models.
// When the primary model fails, the system tries the fallback model.
type ModelFallback struct {
	ID              string         `db:"id" json:"id"`
	ModelID         kernel.ModelID `db:"model_id" json:"model_id"`
	FallbackModelID kernel.ModelID `db:"fallback_model_id" json:"fallback_model_id"`
	Priority        int            `db:"priority" json:"priority"` // lower = higher priority
	Enabled         bool           `db:"enabled" json:"enabled"`
	CreatedAt       time.Time      `db:"created_at" json:"created_at"`
}

// CreateModelFallbackRequest is the DTO for creating a fallback mapping.
type CreateModelFallbackRequest struct {
	ModelID         kernel.ModelID `json:"model_id"`
	FallbackModelID kernel.ModelID `json:"fallback_model_id"`
	Priority        int            `json:"priority"`
}

func (r *CreateModelFallbackRequest) Validate() error {
	if r.ModelID.IsEmpty() {
		return errx.Validation("model_id is required").WithDetail("field", "model_id")
	}
	if r.FallbackModelID.IsEmpty() {
		return errx.Validation("fallback_model_id is required").WithDetail("field", "fallback_model_id")
	}
	if r.ModelID == r.FallbackModelID {
		return errx.Validation("model cannot be its own fallback").WithDetail("field", "fallback_model_id")
	}
	return nil
}
