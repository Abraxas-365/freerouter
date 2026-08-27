package providerkey

import (
	"context"

	"github.com/Abraxas-365/freerouter/internal/kernel"
)

// ProviderKeyRepository defines the contract for provider key persistence
type ProviderKeyRepository interface {
	FindByID(ctx context.Context, id kernel.ProviderKeyID) (*ProviderKey, error)
	FindByProvider(ctx context.Context, providerID kernel.ProviderID) ([]*ProviderKey, error)
	FindByTenant(ctx context.Context, tenantID kernel.TenantID) ([]*ProviderKey, error)
	FindManaged(ctx context.Context) ([]*ProviderKey, error)
	FindActiveByProvider(ctx context.Context, providerID kernel.ProviderID) ([]*ProviderKey, error)
	Save(ctx context.Context, k ProviderKey) error
	Delete(ctx context.Context, id kernel.ProviderKeyID) error
}

// TokenEncryptor handles encryption/decryption of provider key tokens
type TokenEncryptor interface {
	Encrypt(plaintext string) (ciphertext string, err error)
	Decrypt(ciphertext string) (plaintext string, err error)
	Mask(plaintext string) string
	Hash(plaintext string) string
}
