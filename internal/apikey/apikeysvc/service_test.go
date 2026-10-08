package apikeysvc

import (
	"context"
	"testing"

	"github.com/Abraxas-365/freerouter/internal/apikey"
	"github.com/Abraxas-365/freerouter/internal/errx"
)

type fakeStore struct {
	apikey.Store
	revoked []string
}

func (f *fakeStore) List(context.Context) ([]apikey.APIKey, error) {
	return []apikey.APIKey{{ID: "key", ResourceID: "res"}, {ID: "backend", ResourceID: "iam"}}, nil
}

func (f *fakeStore) Find(_ context.Context, id string) (apikey.APIKey, error) {
	switch id {
	case "key":
		return apikey.APIKey{ID: id, ResourceID: "res"}, nil
	case "backend":
		return apikey.APIKey{ID: id, ResourceID: "iam"}, nil
	}
	return apikey.APIKey{}, errx.NotFound("service account not found")
}

func (f *fakeStore) Revoke(_ context.Context, id string) error {
	f.revoked = append(f.revoked, id)
	return nil
}

func TestOnlyFreeRouterKeysAreVisible(t *testing.T) {
	ctx := context.Background()
	store := &fakeStore{}
	svc := New(store, "app", "res")

	keys, err := svc.List(ctx)
	if err != nil || len(keys) != 1 || keys[0].ID != "key" {
		t.Fatalf("List = %v, %v; want only FreeRouter's key", keys, err)
	}

	err = svc.Revoke(ctx, "backend")
	var x *errx.Error
	if !errx.As(err, &x) || x.Type != errx.TypeNotFound {
		t.Fatalf("revoke backend account: err = %v, want NOT_FOUND", err)
	}
	if err := svc.Revoke(ctx, "key"); err != nil {
		t.Fatal(err)
	}
	if len(store.revoked) != 1 || store.revoked[0] != "key" {
		t.Fatalf("revoked = %v, want [key]", store.revoked)
	}
}
