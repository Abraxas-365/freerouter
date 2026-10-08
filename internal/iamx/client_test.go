package iamx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/iamkit/sdk/apiclient"
	"github.com/Abraxas-365/iamkit/sdk/apierror"
)

// fakeIAMKit issues machine tokens tok-1, tok-2, … and serves
// GET /api/v1/environments/env-1/roles, rejecting any token in reject.
type fakeIAMKit struct {
	server   *httptest.Server
	issued   atomic.Int32
	reject   atomic.Value // string: token to answer 401 for
	lastAuth atomic.Value // string
}

func newFake(t *testing.T) *fakeIAMKit {
	t.Helper()
	f := &fakeIAMKit{}
	f.reject.Store("")
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/identity/v1/machine-token":
			if r.Header.Get("Authorization") != "Bearer ik_svc_ok" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":{"code":"UNAUTHORIZED","message":"bad"}}`))
				return
			}
			n := f.issued.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "tok-" + string(rune('0'+n)), "token_type": "Bearer", "expires_in": 900,
			})
		case "/api/v1/environments/env-1/roles":
			auth := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			f.lastAuth.Store(auth)
			if auth == f.reject.Load().(string) {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":{"code":"UNAUTHORIZED","message":"expired"}}`))
				return
			}
			_, _ = w.Write([]byte(`{"items":[],"next_cursor":""}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.server.Close)
	return f
}

func listRoles(ctx context.Context) func(apiclient.Environment) error {
	return func(env apiclient.Environment) error {
		_, err := env.Roles(ctx)
		return err
	}
}

func TestDo_ReusesMachineToken(t *testing.T) {
	f := newFake(t)
	c := New(f.server.URL, "env-1", "ik_svc_ok", nil)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if err := c.Do(ctx, listRoles(ctx)); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	if got := f.issued.Load(); got != 1 {
		t.Errorf("machine tokens issued = %d, want 1 (cached)", got)
	}
	if got := f.lastAuth.Load(); got != "tok-1" {
		t.Errorf("Authorization = %v, want tok-1", got)
	}
}

func TestDo_RenewsOnceOn401(t *testing.T) {
	f := newFake(t)
	c := New(f.server.URL, "env-1", "ik_svc_ok", nil)
	ctx := context.Background()
	if err := c.Do(ctx, listRoles(ctx)); err != nil {
		t.Fatal(err)
	}

	f.reject.Store("tok-1") // the cached token stops being accepted
	if err := c.Do(ctx, listRoles(ctx)); err != nil {
		t.Fatalf("retry with renewed token: %v", err)
	}
	if got := f.issued.Load(); got != 2 {
		t.Errorf("machine tokens issued = %d, want 2", got)
	}
	if got := f.lastAuth.Load(); got != "tok-2" {
		t.Errorf("Authorization = %v, want tok-2", got)
	}
}

func TestDo_BadSecretIsExternalError(t *testing.T) {
	f := newFake(t)
	c := New(f.server.URL, "env-1", "ik_svc_wrong", nil)
	ctx := context.Background()

	err := c.Do(ctx, listRoles(ctx))
	var x *errx.Error
	if !errx.As(err, &x) || x.Type != errx.TypeExternal {
		t.Fatalf("err = %v, want errx EXTERNAL", err)
	}
}

func TestTranslate(t *testing.T) {
	cases := []struct {
		status int
		want   errx.Type
	}{
		{http.StatusBadRequest, errx.TypeValidation},
		{http.StatusNotFound, errx.TypeNotFound},
		{http.StatusConflict, errx.TypeConflict},
		{http.StatusUnprocessableEntity, errx.TypeBusiness},
		{http.StatusForbidden, errx.TypeExternal}, // our credential lacks a permission: ops problem, not the caller's
		{http.StatusInternalServerError, errx.TypeExternal},
	}
	for _, tc := range cases {
		err := Translate(&apierror.Error{Code: "X", Message: "m", HTTPStatus: tc.status}, "op")
		var x *errx.Error
		if !errx.As(err, &x) || x.Type != tc.want {
			t.Errorf("status %d: got %v, want %s", tc.status, err, tc.want)
		}
	}
	if Translate(nil, "op") != nil {
		t.Error("Translate(nil) must be nil")
	}
	already := errx.Validation("kept")
	if Translate(already, "op") != already {
		t.Error("an errx.Error must pass through unchanged")
	}
}
