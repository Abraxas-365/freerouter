package apikeysvc

import (
	"context"

	"github.com/Abraxas-365/freerouter/internal/apikey"
	"github.com/Abraxas-365/freerouter/internal/server"
)

// Resolver turns a gateway key FreeRouter issued (fr_...) into the request
// principal: subject = the key id (what rate-limit and routing configs
// name), permissions = the key's grants. It is the server.KeyResolver of
// the none and token auth modes.
type Resolver struct {
	store apikey.LocalStore
}

// NewResolver creates a key resolver over the local store.
func NewResolver(store apikey.LocalStore) *Resolver { return &Resolver{store: store} }

func (r *Resolver) ResolveKey(ctx context.Context, secret string) (server.Principal, error) {
	key, err := r.store.Resolve(ctx, secret)
	if err != nil {
		return server.Principal{}, err
	}
	return server.Principal{Subject: key.ID, Permissions: key.Permissions, ExpiresAt: key.ExpiresAt}, nil
}

var _ server.KeyResolver = (*Resolver)(nil)
