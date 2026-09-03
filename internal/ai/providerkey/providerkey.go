package providerkey

import (
	"net/http"
	"strings"
	"time"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/kernel"
)

// ============================================================================
// Provider Key Entity
// ============================================================================

// KeyStatus defines the status of a provider key
type KeyStatus string

const (
	KeyStatusActive   KeyStatus = "active"
	KeyStatusInactive KeyStatus = "inactive"
)

// ProviderKey represents a credential for an upstream LLM provider.
// Managed keys are platform-owned (used for credits-mode traffic).
// BYOK keys are tenant-owned (tenant brings their own API key).
type ProviderKey struct {
	ID         kernel.ProviderKeyID `db:"id" json:"id"`
	ProviderID kernel.ProviderID    `db:"provider_id" json:"provider_id"`
	TenantID   *kernel.TenantID     `db:"tenant_id" json:"tenant_id,omitempty"` // nil = managed/platform key

	// Credential (stored encrypted in production; token never returned in DTOs)
	TokenCiphertext string  `db:"token_ciphertext" json:"-"`
	TokenMasked     string  `db:"token_masked" json:"token_masked"`
	TokenHash       string  `db:"token_hash" json:"-"`
	BaseURL         *string `db:"base_url" json:"base_url,omitempty"` // Custom endpoint override

	Name        string    `db:"name" json:"name"`
	Description string    `db:"description" json:"description"`
	Managed     bool      `db:"managed" json:"managed"` // true = platform key, false = BYOK
	Status      KeyStatus `db:"status" json:"status"`
	SortOrder   *int      `db:"sort_order" json:"sort_order,omitempty"` // Lower = higher priority

	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

func (k *ProviderKey) IsActive() bool {
	return k.Status == KeyStatusActive
}

func (k *ProviderKey) IsManaged() bool {
	return k.Managed
}

func (k *ProviderKey) IsBYOK() bool {
	return !k.Managed
}

// ============================================================================
// DTOs
// ============================================================================

// ProviderKeyDTO is the external representation (token is never exposed)
type ProviderKeyDTO struct {
	ID          kernel.ProviderKeyID `json:"id"`
	ProviderID  kernel.ProviderID    `json:"provider_id"`
	TenantID    *kernel.TenantID     `json:"tenant_id,omitempty"`
	TokenMasked string               `json:"token_masked"`
	BaseURL     *string              `json:"base_url,omitempty"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Managed     bool                 `json:"managed"`
	Status      KeyStatus            `json:"status"`
	SortOrder   *int                 `json:"sort_order,omitempty"`
	CreatedAt   time.Time            `json:"created_at"`
}

func (k *ProviderKey) ToDTO() ProviderKeyDTO {
	return ProviderKeyDTO{
		ID:          k.ID,
		ProviderID:  k.ProviderID,
		TenantID:    k.TenantID,
		TokenMasked: k.TokenMasked,
		BaseURL:     k.BaseURL,
		Name:        k.Name,
		Description: k.Description,
		Managed:     k.Managed,
		Status:      k.Status,
		SortOrder:   k.SortOrder,
		CreatedAt:   k.CreatedAt,
	}
}

// ============================================================================
// Request types
// ============================================================================

// CreateProviderKeyRequest for adding a new credential
type CreateProviderKeyRequest struct {
	ProviderID  kernel.ProviderID `json:"provider_id"`
	TenantID    *kernel.TenantID  `json:"tenant_id,omitempty"` // nil = managed key
	Token       string            `json:"token"`
	BaseURL     *string           `json:"base_url,omitempty"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
}

func (r *CreateProviderKeyRequest) Validate() error {
	if r.ProviderID.IsEmpty() {
		return errx.Validation("Provider ID is required").WithDetail("field", "provider_id")
	}
	r.Token = strings.TrimSpace(r.Token)
	if r.Token == "" {
		return errx.Validation("Token is required").WithDetail("field", "token")
	}
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		return errx.Validation("Name is required").WithDetail("field", "name")
	}
	return nil
}

// UpdateProviderKeyRequest for updating a credential
type UpdateProviderKeyRequest struct {
	Token       *string    `json:"token,omitempty"` // Re-encrypt if provided
	BaseURL     *string    `json:"base_url,omitempty"`
	Name        *string    `json:"name,omitempty"`
	Description *string    `json:"description,omitempty"`
	Status      *KeyStatus `json:"status,omitempty"`
	SortOrder   *int       `json:"sort_order,omitempty"`
}

func (r *UpdateProviderKeyRequest) Validate() error {
	if r.Token != nil && strings.TrimSpace(*r.Token) == "" {
		return errx.Validation("Token cannot be empty").WithDetail("field", "token")
	}
	if r.Status != nil {
		switch *r.Status {
		case KeyStatusActive, KeyStatusInactive:
		default:
			return errx.Validation("Invalid key status").WithDetail("field", "status")
		}
	}
	return nil
}

// ============================================================================
// Response types
// ============================================================================

type ProviderKeyListResponse struct {
	Keys  []ProviderKeyDTO `json:"keys"`
	Total int              `json:"total"`
}

// ============================================================================
// Errors
// ============================================================================

var ErrRegistry = errx.NewRegistry("PROVIDER_KEY")

var (
	CodeKeyNotFound      = ErrRegistry.Register("KEY_NOT_FOUND", errx.TypeNotFound, http.StatusNotFound, "Provider key not found")
	CodeKeyAlreadyExists = ErrRegistry.Register("KEY_ALREADY_EXISTS", errx.TypeConflict, http.StatusConflict, "Provider key already exists")
)

func ErrKeyNotFound() *errx.Error     { return ErrRegistry.New(CodeKeyNotFound) }
func ErrKeyAlreadyExists() *errx.Error { return ErrRegistry.New(CodeKeyAlreadyExists) }
