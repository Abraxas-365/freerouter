package apikeypg_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Abraxas-365/freerouter/internal/apikey"
	"github.com/Abraxas-365/freerouter/internal/apikey/adapters/apikeypg"
	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/server"
	"github.com/Abraxas-365/freerouter/internal/testutil"
)

func isType(err error, typ errx.Type) bool {
	var x *errx.Error
	return errx.As(err, &x) && x.Type == typ
}

func TestStore_CreateResolveListRevoke(t *testing.T) {
	db := testutil.PostgresDB(t)
	store := apikeypg.New(db)
	ctx := context.Background()

	before := time.Now()
	cred, err := store.Create(ctx, apikey.CreateServiceAccount{
		Name: "ci", Permissions: []string{server.PermGatewayInvoke, server.PermUsageRead},
	}, "", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !strings.HasPrefix(cred.Secret, server.LocalKeyPrefix) || len(cred.Secret) < 40 {
		t.Fatalf("secret %q, want fr_ + 256 bits", cred.Secret)
	}
	if d := cred.ExpiresAt.Sub(before); d < 23*time.Hour || d > 25*time.Hour {
		t.Fatalf("default expiry %s from now, want ~24h", d)
	}

	var stored int
	if err := db.Get(&stored, `SELECT count(*) FROM gateway_keys WHERE secret_hash = $1`, cred.Secret); err != nil || stored != 0 {
		t.Fatalf("raw secret stored (%d rows, %v); only its hash may be", stored, err)
	}

	key, err := store.Resolve(ctx, cred.Secret)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if key.ID != cred.ID || key.Name != "ci" || len(key.Permissions) != 2 {
		t.Fatalf("resolved %+v", key)
	}
	if _, err := store.Resolve(ctx, cred.Secret+"x"); !isType(err, errx.TypeAuthorization) {
		t.Fatalf("wrong secret: err = %v, want UNAUTHORIZED", err)
	}

	list, err := store.List(ctx)
	if err != nil || len(list) != 1 || list[0].ID != cred.ID || list[0].ExpiresIn != "24h0m0s" || list[0].ExpiresAt == nil {
		t.Fatalf("list = %+v, %v", list, err)
	}
	if found, err := store.Find(ctx, cred.ID); err != nil || found.ID != cred.ID {
		t.Fatalf("find = %+v, %v", found, err)
	}

	if err := store.Revoke(ctx, cred.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := store.Resolve(ctx, cred.Secret); !isType(err, errx.TypeAuthorization) {
		t.Fatalf("revoked key resolves: err = %v", err)
	}
	if _, err := store.Find(ctx, cred.ID); !isType(err, errx.TypeNotFound) {
		t.Fatalf("find revoked: err = %v, want NOT_FOUND", err)
	}
	if err := store.Revoke(ctx, cred.ID); !isType(err, errx.TypeNotFound) {
		t.Fatalf("revoke twice: err = %v, want NOT_FOUND", err)
	}
	if list, _ := store.List(ctx); len(list) != 0 {
		t.Fatalf("revoked key listed: %+v", list)
	}
	if err := store.Revoke(ctx, "not-a-uuid"); !isType(err, errx.TypeNotFound) {
		t.Fatalf("revoke bad id: err = %v, want NOT_FOUND", err)
	}
}

func TestStore_Expiry(t *testing.T) {
	db := testutil.PostgresDB(t)
	store := apikeypg.New(db)
	ctx := context.Background()

	never, err := store.Create(ctx, apikey.CreateServiceAccount{Name: "forever", Permissions: []string{server.PermGatewayInvoke}, ExpiresIn: apikey.NeverExpires}, "", "")
	if err != nil || !never.ExpiresAt.IsZero() {
		t.Fatalf("never: %+v, %v", never, err)
	}
	if _, err := store.Resolve(ctx, never.Secret); err != nil {
		t.Fatalf("never-expiring key: %v", err)
	}

	short, err := store.Create(ctx, apikey.CreateServiceAccount{Name: "short", Permissions: []string{server.PermGatewayInvoke}, ExpiresIn: "1h"}, "", "")
	if err != nil {
		t.Fatalf("create 1h: %v", err)
	}
	if _, err := db.Exec(`UPDATE gateway_keys SET expires_at = NOW() - interval '1 second' WHERE id = $1`, short.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Resolve(ctx, short.Secret); !isType(err, errx.TypeAuthorization) {
		t.Fatalf("expired key resolves: err = %v", err)
	}

	if _, err := store.Create(ctx, apikey.CreateServiceAccount{Name: "bad", ExpiresIn: "30m"}, "", ""); !isType(err, errx.TypeValidation) {
		t.Fatalf("30m: err = %v, want VALIDATION", err)
	}
}
