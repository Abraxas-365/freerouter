package errx_test

import (
	"errors"
	"testing"

	"github.com/Abraxas-365/freerouter/internal/errx"
)

// ── HTTP status mapping ─────────────────────────────────────────────

func TestNew_HTTPStatusMapping(t *testing.T) {
	cases := []struct {
		typ    errx.Type
		status int
	}{
		{errx.TypeValidation, 400},
		{errx.TypeAuthorization, 401},
		{errx.TypeForbidden, 403},
		{errx.TypeNotFound, 404},
		{errx.TypeConflict, 409},
		{errx.TypeBusiness, 422},
		{errx.TypeRateLimited, 429},
		{errx.TypeExternal, 502},
		{errx.TypeInternal, 500},
	}
	for _, c := range cases {
		err := errx.New("msg", c.typ)
		if err.HTTPStatus != c.status {
			t.Errorf("type %s: expected status %d, got %d", c.typ, c.status, err.HTTPStatus)
		}
		if err.Type != c.typ {
			t.Errorf("expected type %s, got %s", c.typ, err.Type)
		}
		if err.Code != string(c.typ) {
			t.Errorf("expected code %s, got %s", c.typ, err.Code)
		}
	}
}

func TestNew_UnknownTypeDefaultsTo500(t *testing.T) {
	err := errx.New("msg", errx.Type("SOMETHING_UNMAPPED"))
	if err.HTTPStatus != 500 {
		t.Fatalf("expected default status 500, got %d", err.HTTPStatus)
	}
}

// ── Semantic constructors ───────────────────────────────────────────

func TestSemanticConstructors(t *testing.T) {
	cases := []struct {
		name   string
		err    *errx.Error
		typ    errx.Type
		status int
	}{
		{"Validation", errx.Validation("bad input"), errx.TypeValidation, 400},
		{"NotFound", errx.NotFound("missing"), errx.TypeNotFound, 404},
		{"Unauthorized", errx.Unauthorized("no auth"), errx.TypeAuthorization, 401},
		{"Forbidden", errx.Forbidden("no perms"), errx.TypeForbidden, 403},
		{"RateLimited", errx.RateLimited("slow down"), errx.TypeRateLimited, 429},
		{"Conflict", errx.Conflict("dup"), errx.TypeConflict, 409},
		{"Business", errx.Business("rule violated"), errx.TypeBusiness, 422},
		{"External", errx.External("upstream failed"), errx.TypeExternal, 502},
		{"Internal", errx.Internal("boom"), errx.TypeInternal, 500},
	}
	for _, c := range cases {
		if c.err.Type != c.typ {
			t.Errorf("%s: expected type %s, got %s", c.name, c.typ, c.err.Type)
		}
		if c.err.HTTPStatus != c.status {
			t.Errorf("%s: expected status %d, got %d", c.name, c.status, c.err.HTTPStatus)
		}
	}
}

// ── Error() / Unwrap() ──────────────────────────────────────────────

func TestError_MessageFormatting(t *testing.T) {
	e := errx.Validation("bad field")
	if got := e.Error(); got != "[VALIDATION] bad field" {
		t.Fatalf("unexpected error string: %q", got)
	}
}

func TestError_MessageFormattingWithWrappedCause(t *testing.T) {
	cause := errors.New("driver: connection refused")
	e := errx.Wrap(cause, "failed to query", errx.TypeInternal)
	if got := e.Error(); got == "" || got == "[INTERNAL] failed to query" {
		t.Fatalf("expected wrapped cause in error string, got %q", got)
	}
}

func TestUnwrap_ReturnsUnderlyingError(t *testing.T) {
	cause := errors.New("root cause")
	e := errx.Wrap(cause, "wrapped", errx.TypeInternal)
	if !errors.Is(e, cause) {
		t.Fatalf("expected errors.Is to find wrapped cause")
	}
}

// ── Wrap ─────────────────────────────────────────────────────────────

func TestWrap_NilReturnsNil(t *testing.T) {
	if err := errx.Wrap(nil, "msg", errx.TypeInternal); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestWrap_PlainErrorGetsRequestedType(t *testing.T) {
	cause := errors.New("plain driver error")
	e := errx.Wrap(cause, "db failure", errx.TypeInternal)
	if e.Type != errx.TypeInternal {
		t.Fatalf("expected TypeInternal, got %s", e.Type)
	}
	if e.HTTPStatus != 500 {
		t.Fatalf("expected 500, got %d", e.HTTPStatus)
	}
	if e.Message != "db failure" {
		t.Fatalf("expected message to be overridden, got %q", e.Message)
	}
}

func TestWrap_PreservesExistingErrxType(t *testing.T) {
	// A NotFound (404) error re-wrapped as TypeInternal must NOT be
	// downgraded to 500 — the original classification wins.
	original := errx.NotFound("provider not found")
	rewrapped := errx.Wrap(original, "failed to load provider", errx.TypeInternal)

	if rewrapped.Type != errx.TypeNotFound {
		t.Fatalf("expected type to remain NotFound, got %s", rewrapped.Type)
	}
	if rewrapped.HTTPStatus != 404 {
		t.Fatalf("expected status to remain 404, got %d", rewrapped.HTTPStatus)
	}
	if rewrapped.Message != "failed to load provider" {
		t.Fatalf("expected new message to be applied, got %q", rewrapped.Message)
	}
}

func TestWrapf_FormatsMessage(t *testing.T) {
	cause := errors.New("cause")
	e := errx.Wrapf(cause, errx.TypeInternal, "failed for id=%d", 42)
	if e.Message != "failed for id=42" {
		t.Fatalf("unexpected message: %q", e.Message)
	}
}

// ── WithDetail ──────────────────────────────────────────────────────

func TestWithDetail_AddsAndChains(t *testing.T) {
	e := errx.Validation("bad input").WithDetail("field", "email").WithDetail("reason", "invalid format")
	if e.Details["field"] != "email" || e.Details["reason"] != "invalid format" {
		t.Fatalf("expected both details to be set, got %+v", e.Details)
	}
}

// ── ToHTTPResponse / MarshalJSON ────────────────────────────────────

func TestToHTTPResponse(t *testing.T) {
	e := errx.NotFound("provider not found").WithDetail("id", "abc")
	resp := e.ToHTTPResponse()

	if resp.Code != "NOT_FOUND" || resp.Type != "NOT_FOUND" || resp.StatusCode != 404 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Details["id"] != "abc" {
		t.Fatalf("expected detail to carry over, got %+v", resp.Details)
	}
}

func TestMarshalJSON_OmitsInternalErrField(t *testing.T) {
	cause := errors.New("sensitive internal detail")
	e := errx.Wrap(cause, "public message", errx.TypeInternal)

	data, err := e.MarshalJSON()
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}
	if got := string(data); contains(got, "sensitive internal detail") {
		t.Fatalf("expected wrapped cause to be excluded from JSON, got %s", got)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (func() bool {
		for i := 0; i+len(substr) <= len(s); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	})()
}
