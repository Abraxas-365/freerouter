package providerkey

import (
	"context"

	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/query"
)

// Commands defines write operations for provider keys.
type Commands interface {
	Create(ctx context.Context, input Create) (identity.ProviderKeyID, error)
	Update(ctx context.Context, key identity.ProviderKeyID, input Update) error
	Delete(ctx context.Context, key identity.ProviderKeyID) error
}

// Queries defines read operations for provider keys.
type Queries interface {
	Find(ctx context.Context, key identity.ProviderKeyID) (ProviderKey, error)
	List(ctx context.Context, filter Filter, page query.Pagination) (query.Paginated[ProviderKey], error)
	// ListActiveByProvider returns all active keys for a provider (no pagination).
	ListActiveByProvider(ctx context.Context, provider identity.ProviderID) ([]ProviderKey, error)
	// DecryptToken returns the plaintext token for a key (used by the gateway).
	DecryptToken(ctx context.Context, key identity.ProviderKeyID) (string, error)
	// DecryptCredential returns the full credential (api_key or oauth) for a key.
	DecryptCredential(ctx context.Context, key identity.ProviderKeyID) (Credential, error)
}

// Repository is the persistence contract for provider keys.
type Repository interface {
	Create(ctx context.Context, key ProviderKey) error
	Find(ctx context.Context, key identity.ProviderKeyID) (ProviderKey, error)
	List(ctx context.Context, filter Filter, page query.Pagination) (query.Paginated[ProviderKey], error)
	ListActiveByProvider(ctx context.Context, provider identity.ProviderID) ([]ProviderKey, error)
	// ListOAuthKeys returns all active OAuth keys (for refresh scanning).
	ListOAuthKeys(ctx context.Context) ([]ProviderKey, error)
	Update(ctx context.Context, key ProviderKey) error
	Delete(ctx context.Context, key identity.ProviderKeyID) error
}

// TokenEncryptor handles encryption, masking, and hashing of API tokens.
type TokenEncryptor interface {
	Encrypt(plaintext string) (ciphertext string, err error)
	Decrypt(ciphertext string) (plaintext string, err error)
	Mask(plaintext string) string
	Hash(plaintext string) string
}

// TokenRefresher performs an OAuth token refresh against the provider's
// token endpoint. Implementations are provider-specific (Anthropic, OpenAI).
type TokenRefresher interface {
	// Refresh exchanges a refresh token for new access + refresh tokens.
	Refresh(ctx context.Context, refreshToken string) (*OAuthData, error)
	// Protocol returns the provider protocol this refresher handles.
	Protocol() string
}
