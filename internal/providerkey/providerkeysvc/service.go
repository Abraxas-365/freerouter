package providerkeysvc

import (
	"context"
	"encoding/json"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/provider"
	"github.com/Abraxas-365/freerouter/internal/providerkey"
	"github.com/Abraxas-365/freerouter/internal/query"
)

// Service implements providerkey.Commands and providerkey.Queries.
type Service struct {
	repo      providerkey.Repository
	encryptor providerkey.TokenEncryptor
	providers provider.Queries // to verify provider exists
}

var _ providerkey.Commands = (*Service)(nil)
var _ providerkey.Queries = (*Service)(nil)

// New creates a provider key service.
func New(repo providerkey.Repository, encryptor providerkey.TokenEncryptor, providers provider.Queries) *Service {
	return &Service{repo: repo, encryptor: encryptor, providers: providers}
}

func (s *Service) Create(ctx context.Context, input providerkey.Create) (identity.ProviderKeyID, error) {
	if err := input.Validate(); err != nil {
		return identity.ProviderKeyID{}, err
	}

	// Verify provider exists
	if _, err := s.providers.Find(ctx, input.ProviderID); err != nil {
		return identity.ProviderKeyID{}, err
	}

	kt := input.EffectiveKeyType()

	var plaintext, masked, hash string
	switch kt {
	case providerkey.KeyTypeOAuth:
		blob, err := json.Marshal(input.OAuthData)
		if err != nil {
			return identity.ProviderKeyID{}, errx.Wrap(err, "failed to marshal oauth data", errx.TypeInternal)
		}
		plaintext = string(blob)
		masked = "oauth:****" + input.OAuthData.AccessToken[max(0, len(input.OAuthData.AccessToken)-4):]
		hash = s.encryptor.Hash(input.OAuthData.AccessToken)
	default: // api_key
		plaintext = input.Token
		masked = s.encryptor.Mask(input.Token)
		hash = s.encryptor.Hash(input.Token)
	}

	ciphertext, err := s.encryptor.Encrypt(plaintext)
	if err != nil {
		return identity.ProviderKeyID{}, errx.Wrap(err, "failed to encrypt token", errx.TypeInternal)
	}

	id := identity.NewProviderKeyID()
	k := providerkey.ProviderKey{
		ID:              id,
		ProviderID:      input.ProviderID,
		KeyType:         kt,
		TokenCiphertext: ciphertext,
		TokenMasked:     masked,
		TokenHash:       hash,
		BaseURL:         input.BaseURL,
		Name:            input.Name,
		Description:     input.Description,
		Status:          providerkey.KeyStatusActive,
	}

	if err := s.repo.Create(ctx, k); err != nil {
		return identity.ProviderKeyID{}, err
	}
	return id, nil
}

func (s *Service) Find(ctx context.Context, id identity.ProviderKeyID) (providerkey.ProviderKey, error) {
	return s.repo.Find(ctx, id)
}

func (s *Service) List(ctx context.Context, filter providerkey.Filter, page query.Pagination) (query.Paginated[providerkey.ProviderKey], error) {
	return s.repo.List(ctx, filter, page.Normalize())
}

func (s *Service) ListActiveByProvider(ctx context.Context, provider identity.ProviderID) ([]providerkey.ProviderKey, error) {
	return s.repo.ListActiveByProvider(ctx, provider)
}

func (s *Service) Update(ctx context.Context, id identity.ProviderKeyID, input providerkey.Update) error {
	if err := input.Validate(); err != nil {
		return err
	}

	k, err := s.repo.Find(ctx, id)
	if err != nil {
		return err
	}

	// Re-encrypt if token changed (api_key)
	if input.Token != nil {
		if k.KeyType == providerkey.KeyTypeOAuth {
			return errx.Validation("cannot set token on an oauth key; use oauth_data")
		}
		ciphertext, err := s.encryptor.Encrypt(*input.Token)
		if err != nil {
			return errx.Wrap(err, "failed to encrypt token", errx.TypeInternal)
		}
		k.TokenCiphertext = ciphertext
		k.TokenMasked = s.encryptor.Mask(*input.Token)
		k.TokenHash = s.encryptor.Hash(*input.Token)
	}
	// Re-encrypt if oauth data changed
	if input.OAuthData != nil {
		if k.KeyType != providerkey.KeyTypeOAuth {
			return errx.Validation("cannot set oauth_data on an api_key key; use token")
		}
		blob, err := json.Marshal(input.OAuthData)
		if err != nil {
			return errx.Wrap(err, "failed to marshal oauth data", errx.TypeInternal)
		}
		ciphertext, err := s.encryptor.Encrypt(string(blob))
		if err != nil {
			return errx.Wrap(err, "failed to encrypt oauth data", errx.TypeInternal)
		}
		k.TokenCiphertext = ciphertext
		k.TokenMasked = "oauth:****" + input.OAuthData.AccessToken[max(0, len(input.OAuthData.AccessToken)-4):]
		k.TokenHash = s.encryptor.Hash(input.OAuthData.AccessToken)
	}
	if input.BaseURL != nil {
		k.BaseURL = input.BaseURL
	}
	if input.Name != nil {
		k.Name = *input.Name
	}
	if input.Description != nil {
		k.Description = *input.Description
	}
	if input.Status != nil {
		k.Status = *input.Status
	}
	if input.SortOrder != nil {
		k.SortOrder = input.SortOrder
	}

	return s.repo.Update(ctx, k)
}

func (s *Service) Delete(ctx context.Context, id identity.ProviderKeyID) error {
	return s.repo.Delete(ctx, id)
}

func (s *Service) DecryptToken(ctx context.Context, id identity.ProviderKeyID) (string, error) {
	k, err := s.repo.Find(ctx, id)
	if err != nil {
		return "", err
	}
	if k.KeyType == providerkey.KeyTypeOAuth {
		// For OAuth keys, decrypt and return the access_token.
		cred, err := s.decryptCredentialFromKey(k)
		if err != nil {
			return "", err
		}
		return cred.OAuth.AccessToken, nil
	}
	plaintext, err := s.encryptor.Decrypt(k.TokenCiphertext)
	if err != nil {
		return "", errx.Wrap(err, "failed to decrypt token", errx.TypeInternal)
	}
	return plaintext, nil
}

func (s *Service) DecryptCredential(ctx context.Context, id identity.ProviderKeyID) (providerkey.Credential, error) {
	k, err := s.repo.Find(ctx, id)
	if err != nil {
		return providerkey.Credential{}, err
	}
	return s.decryptCredentialFromKey(k)
}

func (s *Service) decryptCredentialFromKey(k providerkey.ProviderKey) (providerkey.Credential, error) {
	plaintext, err := s.encryptor.Decrypt(k.TokenCiphertext)
	if err != nil {
		return providerkey.Credential{}, errx.Wrap(err, "failed to decrypt credential", errx.TypeInternal)
	}

	switch k.KeyType {
	case providerkey.KeyTypeOAuth:
		var data providerkey.OAuthData
		if err := json.Unmarshal([]byte(plaintext), &data); err != nil {
			return providerkey.Credential{}, errx.Wrap(err, "failed to unmarshal oauth data", errx.TypeInternal)
		}
		return providerkey.Credential{KeyType: providerkey.KeyTypeOAuth, OAuth: &data}, nil
	default:
		return providerkey.Credential{KeyType: providerkey.KeyTypeAPIKey, Token: plaintext}, nil
	}
}
