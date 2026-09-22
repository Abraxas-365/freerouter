package providerkey

import (
	"strings"
	"time"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/identity"
)

// ── Key Status ──────────────────────────────────────────────────────

type KeyStatus string

const (
	KeyStatusActive   KeyStatus = "active"
	KeyStatusInactive KeyStatus = "inactive"
)

func (s KeyStatus) Valid() bool {
	return s == KeyStatusActive || s == KeyStatusInactive
}

// ── Key Type ────────────────────────────────────────────────────────

// KeyType distinguishes API-key credentials from OAuth credentials.
type KeyType string

const (
	KeyTypeAPIKey KeyType = "api_key" // static API key (default)
	KeyTypeOAuth  KeyType = "oauth"   // OAuth access + refresh tokens
)

func (t KeyType) Valid() bool {
	return t == KeyTypeAPIKey || t == KeyTypeOAuth
}

// ── OAuthData ───────────────────────────────────────────────────────

// OAuthData holds the OAuth tokens for a key of type "oauth".
// Serialised to JSON and stored encrypted in token_ciphertext.
type OAuthData struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// Credential is the decrypted result returned to the gateway.
// For api_key keys, Token is set; for oauth keys, OAuth is set.
type Credential struct {
	KeyType KeyType
	Token   string     // non-empty for api_key
	OAuth   *OAuthData // non-nil for oauth
}

// ── ProviderKey ─────────────────────────────────────────────────────

// ProviderKey is an encrypted upstream API credential for a provider.
type ProviderKey struct {
	ID              identity.ProviderKeyID `json:"id"               db:"id"`
	ProviderID      identity.ProviderID    `json:"provider_id"      db:"provider_id"`
	KeyType         KeyType                `json:"key_type"         db:"key_type"`
	TokenCiphertext string                 `json:"-"                db:"token_ciphertext"`
	TokenMasked     string                 `json:"token_masked"     db:"token_masked"`
	TokenHash       string                 `json:"-"                db:"token_hash"`
	BaseURL         *string                `json:"base_url,omitempty" db:"base_url"`
	Name            string                 `json:"name"             db:"name"`
	Description     string                 `json:"description"      db:"description"`
	Status          KeyStatus              `json:"status"           db:"status"`
	SortOrder       *int                   `json:"sort_order,omitempty" db:"sort_order"`
	CreatedAt       time.Time              `json:"created_at"       db:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"       db:"updated_at"`
}

// ── Create ──────────────────────────────────────────────────────────

// Create holds the data needed to register a new provider key.
type Create struct {
	ProviderID  identity.ProviderID `json:"provider_id"`
	KeyType     KeyType             `json:"key_type"`
	Token       string              `json:"token,omitempty"`        // required for api_key
	OAuthData   *OAuthData          `json:"oauth_data,omitempty"`   // required for oauth
	BaseURL     *string             `json:"base_url,omitempty"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
}

func (c Create) Validate() error {
	if c.ProviderID.IsZero() {
		return errx.Validation("provider_id is required")
	}
	if strings.TrimSpace(c.Name) == "" {
		return errx.Validation("name is required")
	}

	kt := c.effectiveKeyType()
	switch kt {
	case KeyTypeAPIKey:
		if strings.TrimSpace(c.Token) == "" {
			return errx.Validation("token is required for api_key keys")
		}
	case KeyTypeOAuth:
		if c.OAuthData == nil {
			return errx.Validation("oauth_data is required for oauth keys")
		}
		if strings.TrimSpace(c.OAuthData.AccessToken) == "" {
			return errx.Validation("oauth_data.access_token is required")
		}
		if strings.TrimSpace(c.OAuthData.RefreshToken) == "" {
			return errx.Validation("oauth_data.refresh_token is required")
		}
		if c.OAuthData.ExpiresAt.IsZero() {
			return errx.Validation("oauth_data.expires_at is required")
		}
	default:
		return errx.Validation("invalid key_type: must be api_key or oauth")
	}
	return nil
}

// effectiveKeyType returns the key type, defaulting to api_key for
// backward compatibility when the field is empty.
func (c Create) effectiveKeyType() KeyType {
	if c.KeyType == "" {
		return KeyTypeAPIKey
	}
	return c.KeyType
}

// EffectiveKeyType is the exported accessor (used by service).
func (c Create) EffectiveKeyType() KeyType { return c.effectiveKeyType() }

// ── Update ──────────────────────────────────────────────────────────

// Update holds optional fields to patch a provider key.
type Update struct {
	Token       *string    `json:"token,omitempty"`
	OAuthData   *OAuthData `json:"oauth_data,omitempty"`
	BaseURL     *string    `json:"base_url,omitempty"`
	Name        *string    `json:"name,omitempty"`
	Description *string    `json:"description,omitempty"`
	Status      *KeyStatus `json:"status,omitempty"`
	SortOrder   *int       `json:"sort_order,omitempty"`
}

func (u Update) Validate() error {
	if u.Token != nil && strings.TrimSpace(*u.Token) == "" {
		return errx.Validation("token cannot be empty")
	}
	if u.OAuthData != nil {
		if strings.TrimSpace(u.OAuthData.AccessToken) == "" {
			return errx.Validation("oauth_data.access_token cannot be empty")
		}
		if strings.TrimSpace(u.OAuthData.RefreshToken) == "" {
			return errx.Validation("oauth_data.refresh_token cannot be empty")
		}
		if u.OAuthData.ExpiresAt.IsZero() {
			return errx.Validation("oauth_data.expires_at is required")
		}
	}
	if u.Token != nil && u.OAuthData != nil {
		return errx.Validation("cannot set both token and oauth_data")
	}
	if u.Name != nil && strings.TrimSpace(*u.Name) == "" {
		return errx.Validation("name cannot be empty")
	}
	if u.Status != nil && !u.Status.Valid() {
		return errx.Validation("invalid key status")
	}
	return nil
}

// ── Filter ──────────────────────────────────────────────────────────

// Filter holds criteria for listing provider keys.
type Filter struct {
	ProviderID *identity.ProviderID
	Status     *KeyStatus
	KeyType    *KeyType
}
