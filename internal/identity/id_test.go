package identity_test

import (
	"encoding/json"
	"testing"

	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/google/uuid"
)

// ── New / IsZero ────────────────────────────────────────────────────

func TestNewProviderID_IsNotZero(t *testing.T) {
	id := identity.NewProviderID()
	if id.IsZero() {
		t.Fatalf("expected freshly generated ID to be non-zero")
	}
}

func TestZeroValue_IsZero(t *testing.T) {
	var id identity.ProviderID
	if !id.IsZero() {
		t.Fatalf("expected zero value to report IsZero() == true")
	}
}

func TestNewProviderID_GeneratesUniqueIDs(t *testing.T) {
	a := identity.NewProviderID()
	b := identity.NewProviderID()
	if a.String() == b.String() {
		t.Fatalf("expected two calls to NewProviderID to produce different IDs")
	}
}

// ── Parse / round-trip ──────────────────────────────────────────────

func TestParseProviderID_RoundTrip(t *testing.T) {
	original := identity.NewProviderID()
	parsed, err := identity.ParseProviderID(original.String())
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if parsed.String() != original.String() {
		t.Fatalf("expected round-trip to preserve value: got %s want %s", parsed, original)
	}
}

func TestParseProviderID_RejectsInvalidUUID(t *testing.T) {
	_, err := identity.ParseProviderID("not-a-uuid")
	if err == nil {
		t.Fatalf("expected error for invalid UUID string")
	}
}

func TestParseProviderID_RejectsEmptyString(t *testing.T) {
	_, err := identity.ParseProviderID("")
	if err == nil {
		t.Fatalf("expected error for empty string")
	}
}

func TestMustParseProviderID_PanicsOnInvalidInput(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic for invalid UUID")
		}
	}()
	identity.MustParseProviderID("garbage")
}

func TestMustParseProviderID_ReturnsValueOnValidInput(t *testing.T) {
	valid := uuid.New().String()
	id := identity.MustParseProviderID(valid)
	if id.String() != valid {
		t.Fatalf("expected %s, got %s", valid, id.String())
	}
}

// ── Cross-type safety (compile-time, verified via distinct String output) ──

func TestDistinctEntityTypes_ProduceIndependentIDs(t *testing.T) {
	// ProviderID and ModelID are distinct instantiations of ID[T]; parsing
	// the same UUID string into each should work independently without
	// cross-contamination.
	raw := uuid.New().String()

	providerID, err := identity.ParseProviderID(raw)
	if err != nil {
		t.Fatalf("unexpected error parsing provider id: %v", err)
	}
	modelID, err := identity.ParseModelID(raw)
	if err != nil {
		t.Fatalf("unexpected error parsing model id: %v", err)
	}

	if providerID.String() != modelID.String() {
		t.Fatalf("expected same underlying UUID string for both typed IDs")
	}
	// The point of phantom types is compile-time separation — this is
	// implicitly verified by the fact both ParseXxxID functions have
	// distinct signatures returning distinct instantiated types.
}

// ── JSON marshal/unmarshal (TextMarshaler/TextUnmarshaler) ──────────

func TestMarshalJSON_NonZero(t *testing.T) {
	id := identity.NewProviderID()
	data, err := json.Marshal(id)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}
	expected := `"` + id.String() + `"`
	if string(data) != expected {
		t.Fatalf("expected %s, got %s", expected, string(data))
	}
}

func TestMarshalJSON_ZeroValueMarshalsEmptyString(t *testing.T) {
	var id identity.ProviderID
	data, err := json.Marshal(id)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}
	if string(data) != `""` {
		t.Fatalf(`expected empty string, got %s`, string(data))
	}
}

func TestUnmarshalJSON_RoundTrip(t *testing.T) {
	original := identity.NewProviderID()
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}

	var decoded identity.ProviderID
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	if decoded.String() != original.String() {
		t.Fatalf("expected round-trip to preserve value")
	}
}

func TestUnmarshalJSON_EmptyStringYieldsZeroValue(t *testing.T) {
	var id identity.ProviderID
	if err := json.Unmarshal([]byte(`""`), &id); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	if !id.IsZero() {
		t.Fatalf("expected zero value after unmarshaling empty string")
	}
}

func TestUnmarshalJSON_InvalidUUIDReturnsError(t *testing.T) {
	var id identity.ProviderID
	err := json.Unmarshal([]byte(`"not-a-uuid"`), &id)
	if err == nil {
		t.Fatalf("expected error unmarshaling invalid UUID")
	}
}

// ── database/sql Valuer/Scanner ──────────────────────────────────────

func TestValue_NonZeroReturnsString(t *testing.T) {
	id := identity.NewProviderID()
	v, err := id.Value()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != id.String() {
		t.Fatalf("expected %s, got %v", id.String(), v)
	}
}

func TestValue_ZeroReturnsNil(t *testing.T) {
	var id identity.ProviderID
	v, err := id.Value()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != nil {
		t.Fatalf("expected nil driver value for zero ID, got %v", v)
	}
}

func TestScan_FromString(t *testing.T) {
	raw := uuid.New().String()
	var id identity.ProviderID
	if err := id.Scan(raw); err != nil {
		t.Fatalf("unexpected scan error: %v", err)
	}
	if id.String() != raw {
		t.Fatalf("expected %s, got %s", raw, id.String())
	}
}

func TestScan_FromBytes(t *testing.T) {
	raw := uuid.New().String()
	var id identity.ProviderID
	if err := id.Scan([]byte(raw)); err != nil {
		t.Fatalf("unexpected scan error: %v", err)
	}
	if id.String() != raw {
		t.Fatalf("expected %s, got %s", raw, id.String())
	}
}

func TestScan_Nil(t *testing.T) {
	id := identity.NewProviderID()
	if err := id.Scan(nil); err != nil {
		t.Fatalf("unexpected scan error: %v", err)
	}
	if !id.IsZero() {
		t.Fatalf("expected scanning nil to zero out the ID")
	}
}

func TestScan_InvalidTypeReturnsError(t *testing.T) {
	var id identity.ProviderID
	if err := id.Scan(12345); err == nil {
		t.Fatalf("expected error scanning unsupported type")
	}
}

func TestScan_InvalidStringReturnsError(t *testing.T) {
	var id identity.ProviderID
	if err := id.Scan("not-a-uuid"); err == nil {
		t.Fatalf("expected error scanning invalid UUID string")
	}
}

// ── UUID() accessor ──────────────────────────────────────────────────

func TestUUID_ReturnsUnderlyingValue(t *testing.T) {
	raw := uuid.New()
	id := identity.MustParseProviderID(raw.String())
	if id.UUID() != raw {
		t.Fatalf("expected underlying uuid.UUID to match")
	}
}
