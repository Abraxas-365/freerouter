// Package apikeypg implements apikey.LocalStore in PostgreSQL: the gateway
// keys FreeRouter issues itself when it runs without IAMKit
// (FREEROUTER_AUTH=none|token). Only the SHA-256 of a secret is stored.
package apikeypg

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"time"

	"github.com/Abraxas-365/freerouter/internal/apikey"
	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/server"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// Store implements apikey.LocalStore over the gateway_keys table.
type Store struct {
	db  *sqlx.DB
	now func() time.Time
}

var _ apikey.LocalStore = (*Store)(nil)

// New creates a PostgreSQL-backed gateway key store.
func New(db *sqlx.DB) *Store { return &Store{db: db, now: time.Now} }

type row struct {
	ID          identity.GatewayKeyID `db:"id"`
	Name        string                `db:"name"`
	Permissions pq.StringArray        `db:"permissions"`
	ExpiresIn   string                `db:"expires_in"`
	ExpiresAt   *time.Time            `db:"expires_at"`
}

func (r row) account() apikey.ServiceAccount {
	return apikey.ServiceAccount{ID: r.ID.String(), Name: r.Name, Permissions: []string(r.Permissions),
		ExpiresIn: r.ExpiresIn, ExpiresAt: r.ExpiresAt}
}

const columns = `id, name, permissions, expires_in, expires_at`

// Create issues a new key. applicationID and resourceID are IAMKit
// boundaries and have no meaning here.
func (s *Store) Create(ctx context.Context, input apikey.CreateServiceAccount, _, _ string) (apikey.ServiceAccountCredential, error) {
	now := s.now().UTC()
	expiresAt, err := apikey.ExpiryFrom(input.ExpiresIn, now)
	if err != nil {
		return apikey.ServiceAccountCredential{}, err
	}
	secret, err := newSecret()
	if err != nil {
		return apikey.ServiceAccountCredential{}, errx.Wrap(err, "failed to generate gateway key", errx.TypeInternal)
	}
	id := identity.NewGatewayKeyID()
	expiresIn := input.ExpiresIn
	if expiresIn == "" {
		expiresIn = apikey.DefaultExpiry.String()
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO gateway_keys (id, name, secret_hash, permissions, expires_in, expires_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		id, input.Name, hash(secret), pq.Array(input.Permissions), expiresIn, expiresAt, now)
	if err != nil {
		return apikey.ServiceAccountCredential{}, errx.Wrap(err, "failed to create gateway key", errx.TypeInternal)
	}
	cred := apikey.ServiceAccountCredential{ID: id.String(), Secret: secret}
	if expiresAt != nil {
		cred.ExpiresAt = *expiresAt
	}
	return cred, nil
}

// List returns the keys that are not revoked (expired ones included, so
// the console shows them until revoked).
func (s *Store) List(ctx context.Context) ([]apikey.ServiceAccount, error) {
	var rows []row
	if err := s.db.SelectContext(ctx, &rows,
		`SELECT `+columns+` FROM gateway_keys WHERE revoked_at IS NULL ORDER BY created_at DESC`); err != nil {
		return nil, errx.Wrap(err, "failed to list gateway keys", errx.TypeInternal)
	}
	out := make([]apikey.ServiceAccount, len(rows))
	for i, r := range rows {
		out[i] = r.account()
	}
	return out, nil
}

// Find returns a key that is not revoked.
func (s *Store) Find(ctx context.Context, id string) (apikey.ServiceAccount, error) {
	key, err := identity.ParseGatewayKeyID(id)
	if err != nil {
		return apikey.ServiceAccount{}, errx.NotFound("service account not found")
	}
	var r row
	err = s.db.GetContext(ctx, &r, `SELECT `+columns+` FROM gateway_keys WHERE id = $1 AND revoked_at IS NULL`, key)
	switch {
	case err == sql.ErrNoRows:
		return apikey.ServiceAccount{}, errx.NotFound("service account not found")
	case err != nil:
		return apikey.ServiceAccount{}, errx.Wrap(err, "failed to find gateway key", errx.TypeInternal)
	}
	return r.account(), nil
}

// ListApplications: IAMKit applications don't exist without IAMKit.
func (s *Store) ListApplications(context.Context) ([]apikey.Application, error) {
	return []apikey.Application{}, nil
}

// Revoke marks the key revoked; it stops working on the next request.
func (s *Store) Revoke(ctx context.Context, id string) error {
	key, err := identity.ParseGatewayKeyID(id)
	if err != nil {
		return errx.NotFound("service account not found")
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE gateway_keys SET revoked_at = $2 WHERE id = $1 AND revoked_at IS NULL`, key, s.now().UTC())
	if err != nil {
		return errx.Wrap(err, "failed to revoke gateway key", errx.TypeInternal)
	}
	if n, err := res.RowsAffected(); err != nil {
		return errx.Wrap(err, "failed to revoke gateway key", errx.TypeInternal)
	} else if n == 0 {
		return errx.NotFound("service account not found")
	}
	return nil
}

// Resolve returns the live key whose secret is presented.
func (s *Store) Resolve(ctx context.Context, secret string) (apikey.ServiceAccount, error) {
	var r row
	err := s.db.GetContext(ctx, &r,
		`SELECT `+columns+` FROM gateway_keys
		 WHERE secret_hash = $1 AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > $2)`,
		hash(secret), s.now().UTC())
	switch {
	case err == sql.ErrNoRows:
		return apikey.ServiceAccount{}, errx.Unauthorized("invalid or expired token")
	case err != nil:
		return apikey.ServiceAccount{}, errx.Wrap(err, "failed to resolve gateway key", errx.TypeInternal)
	}
	return r.account(), nil
}

// newSecret is fr_ + 32 random bytes (base64url): 256 bits, so a plain
// SHA-256 (no salt, no KDF) is enough to store it.
func newSecret() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return server.LocalKeyPrefix + base64.RawURLEncoding.EncodeToString(b[:]), nil
}

func hash(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}
