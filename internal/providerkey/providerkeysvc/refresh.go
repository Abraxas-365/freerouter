package providerkeysvc

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/provider"
	"github.com/Abraxas-365/freerouter/internal/providerkey"
)

const (
	// refreshBuffer is how far in advance of expiry to refresh.
	refreshBuffer = 5 * time.Minute
	// pollInterval is how often the background worker checks for expiring tokens.
	pollInterval = 1 * time.Minute
)

// RefreshService handles background and on-demand OAuth token refresh.
type RefreshService struct {
	repo       providerkey.Repository
	encryptor  providerkey.TokenEncryptor
	providers  provider.Queries
	refreshers map[string]providerkey.TokenRefresher // protocol → refresher

	// Per-key mutex to avoid concurrent refreshes of the same key.
	mu    sync.Mutex
	locks map[identity.ProviderKeyID]*sync.Mutex

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewRefreshService creates a token refresh service.
func NewRefreshService(
	repo providerkey.Repository,
	encryptor providerkey.TokenEncryptor,
	providers provider.Queries,
	refreshers []providerkey.TokenRefresher,
) *RefreshService {
	rm := make(map[string]providerkey.TokenRefresher, len(refreshers))
	for _, r := range refreshers {
		rm[r.Protocol()] = r
	}
	return &RefreshService{
		repo:       repo,
		encryptor:  encryptor,
		providers:  providers,
		refreshers: rm,
		locks:      make(map[identity.ProviderKeyID]*sync.Mutex),
		stopCh:     make(chan struct{}),
	}
}

// StartWorker starts the background refresh loop.
func (s *RefreshService) StartWorker() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-s.stopCh:
				return
			case <-ticker.C:
				s.refreshExpiring()
			}
		}
	}()
}

// Stop gracefully stops the background worker.
func (s *RefreshService) Stop() {
	close(s.stopCh)
	s.wg.Wait()
}

// RefreshIfNeeded checks whether an OAuth key needs refresh and does it.
// Safe to call from the gateway hot path (e.g. after a 401).
// Returns the updated credential. For api_key keys, this is a no-op.
func (s *RefreshService) RefreshIfNeeded(ctx context.Context, keyID identity.ProviderKeyID) (providerkey.Credential, error) {
	k, err := s.repo.Find(ctx, keyID)
	if err != nil {
		return providerkey.Credential{}, err
	}
	if k.KeyType != providerkey.KeyTypeOAuth {
		// api_key — just decrypt and return.
		plaintext, err := s.encryptor.Decrypt(k.TokenCiphertext)
		if err != nil {
			return providerkey.Credential{}, errx.Wrap(err, "failed to decrypt token", errx.TypeInternal)
		}
		return providerkey.Credential{KeyType: providerkey.KeyTypeAPIKey, Token: plaintext}, nil
	}

	// Decrypt current OAuth data.
	plaintext, err := s.encryptor.Decrypt(k.TokenCiphertext)
	if err != nil {
		return providerkey.Credential{}, errx.Wrap(err, "failed to decrypt oauth data", errx.TypeInternal)
	}
	var data providerkey.OAuthData
	if err := json.Unmarshal([]byte(plaintext), &data); err != nil {
		return providerkey.Credential{}, errx.Wrap(err, "failed to unmarshal oauth data", errx.TypeInternal)
	}

	// If not expiring soon, return as-is.
	if time.Until(data.ExpiresAt) > refreshBuffer {
		return providerkey.Credential{KeyType: providerkey.KeyTypeOAuth, OAuth: &data}, nil
	}

	// Needs refresh — do it under a per-key lock.
	updated, err := s.doRefresh(ctx, k, data)
	if err != nil {
		return providerkey.Credential{}, err
	}
	return providerkey.Credential{KeyType: providerkey.KeyTypeOAuth, OAuth: updated}, nil
}

// ForceRefresh unconditionally refreshes an OAuth key (e.g. after a 401).
func (s *RefreshService) ForceRefresh(ctx context.Context, keyID identity.ProviderKeyID) (providerkey.Credential, error) {
	k, err := s.repo.Find(ctx, keyID)
	if err != nil {
		return providerkey.Credential{}, err
	}
	if k.KeyType != providerkey.KeyTypeOAuth {
		return providerkey.Credential{}, errx.Validation("cannot refresh a non-oauth key")
	}

	plaintext, err := s.encryptor.Decrypt(k.TokenCiphertext)
	if err != nil {
		return providerkey.Credential{}, errx.Wrap(err, "failed to decrypt oauth data", errx.TypeInternal)
	}
	var data providerkey.OAuthData
	if err := json.Unmarshal([]byte(plaintext), &data); err != nil {
		return providerkey.Credential{}, errx.Wrap(err, "failed to unmarshal oauth data", errx.TypeInternal)
	}

	updated, err := s.doRefresh(ctx, k, data)
	if err != nil {
		return providerkey.Credential{}, err
	}
	return providerkey.Credential{KeyType: providerkey.KeyTypeOAuth, OAuth: updated}, nil
}

// doRefresh performs the actual refresh under a per-key lock, persists, and returns the new data.
func (s *RefreshService) doRefresh(ctx context.Context, k providerkey.ProviderKey, current providerkey.OAuthData) (*providerkey.OAuthData, error) {
	lock := s.keyLock(k.ID)
	lock.Lock()
	defer lock.Unlock()

	// Double-check: re-read from DB in case another goroutine already refreshed.
	k2, err := s.repo.Find(ctx, k.ID)
	if err != nil {
		return nil, err
	}
	plaintext, err := s.encryptor.Decrypt(k2.TokenCiphertext)
	if err != nil {
		return nil, errx.Wrap(err, "failed to decrypt oauth data", errx.TypeInternal)
	}
	var fresh providerkey.OAuthData
	if err := json.Unmarshal([]byte(plaintext), &fresh); err != nil {
		return nil, errx.Wrap(err, "failed to unmarshal oauth data", errx.TypeInternal)
	}
	// If the access token changed since our read, someone else refreshed.
	if fresh.AccessToken != current.AccessToken && time.Until(fresh.ExpiresAt) > refreshBuffer {
		return &fresh, nil
	}

	// Look up the provider to find the protocol.
	prov, err := s.providers.Find(ctx, k.ProviderID)
	if err != nil {
		return nil, err
	}
	refresher, ok := s.refreshers[string(prov.Protocol)]
	if !ok {
		return nil, errx.New("no token refresher registered for provider protocol "+string(prov.Protocol), errx.TypeInternal)
	}

	// Call the provider-specific refresh endpoint.
	newData, err := refresher.Refresh(ctx, fresh.RefreshToken)
	if err != nil {
		return nil, errx.Wrap(err, "oauth token refresh failed", errx.TypeExternal)
	}

	// Persist the new tokens.
	if err := s.persistOAuthData(ctx, k2, newData); err != nil {
		return nil, err
	}

	return newData, nil
}

func (s *RefreshService) persistOAuthData(ctx context.Context, k providerkey.ProviderKey, data *providerkey.OAuthData) error {
	blob, err := json.Marshal(data)
	if err != nil {
		return errx.Wrap(err, "failed to marshal oauth data", errx.TypeInternal)
	}
	ciphertext, err := s.encryptor.Encrypt(string(blob))
	if err != nil {
		return errx.Wrap(err, "failed to encrypt oauth data", errx.TypeInternal)
	}

	k.TokenCiphertext = ciphertext
	k.TokenMasked = "oauth:****" + data.AccessToken[max(0, len(data.AccessToken)-4):]
	k.TokenHash = s.encryptor.Hash(data.AccessToken)

	return s.repo.Update(ctx, k)
}

func (s *RefreshService) keyLock(id identity.ProviderKeyID) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	if l, ok := s.locks[id]; ok {
		return l
	}
	l := &sync.Mutex{}
	s.locks[id] = l
	return l
}

// refreshExpiring scans all active OAuth keys and refreshes those nearing expiry.
func (s *RefreshService) refreshExpiring() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	keys, err := s.repo.ListOAuthKeys(ctx)
	if err != nil {
		log.Printf("oauth refresh: failed to list keys: %v", err)
		return
	}

	for _, k := range keys {
		plaintext, err := s.encryptor.Decrypt(k.TokenCiphertext)
		if err != nil {
			log.Printf("oauth refresh: failed to decrypt key %s: %v", k.ID, err)
			continue
		}
		var data providerkey.OAuthData
		if err := json.Unmarshal([]byte(plaintext), &data); err != nil {
			log.Printf("oauth refresh: failed to unmarshal key %s: %v", k.ID, err)
			continue
		}

		if time.Until(data.ExpiresAt) > refreshBuffer {
			continue // not expiring soon
		}

		log.Printf("oauth refresh: refreshing key %s (expires %s)", k.ID, data.ExpiresAt.Format(time.RFC3339))
		if _, err := s.doRefresh(ctx, k, data); err != nil {
			log.Printf("oauth refresh: failed to refresh key %s: %v", k.ID, err)
		}
	}
}
