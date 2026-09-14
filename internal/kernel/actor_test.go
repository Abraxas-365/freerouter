package kernel

import (
	"encoding/json"
	"testing"
)

func TestActorIdentity(t *testing.T) {
	user := NewUserActor(NewUserID("user-1"))
	key := NewAPIKeyActor(NewAPIKeyID("key-1"))
	if id, ok := user.UserID(); !ok || id.String() != "user-1" {
		t.Fatal("missing user identity")
	}
	if _, ok := user.APIKeyID(); ok {
		t.Fatal("user must not identify as key")
	}
	if id, ok := key.APIKeyID(); !ok || id.String() != "key-1" {
		t.Fatal("missing key identity")
	}
	if _, ok := key.UserID(); ok {
		t.Fatal("key must not impersonate its creator")
	}
	for _, actor := range []Actor{{}, NewUserActor(""), NewAPIKeyActor(""), {userID: "user", apiKeyID: "key"}} {
		if actor.IsValid() {
			t.Fatal("ambiguous or empty actor is valid")
		}
		if _, err := json.Marshal(actor); err == nil {
			t.Fatal("invalid actor serialized")
		}
	}
	encoded, err := json.Marshal(key)
	if err != nil || string(encoded) != `{"type":"api_key","id":"key-1"}` {
		t.Fatalf("unexpected audit identity: %s %v", encoded, err)
	}
	if (&AuthContext{TenantID: "tenant"}).IsValid() {
		t.Fatal("missing actor accepted")
	}
	if (&AuthContext{Actor: key}).IsValid() {
		t.Fatal("missing tenant accepted")
	}
	if !(&AuthContext{Actor: key, TenantID: "tenant"}).IsValid() {
		t.Fatal("valid key context rejected")
	}
}

func TestTenantScopeMatching(t *testing.T) {
	for _, tc := range []struct {
		held, required string
		want           bool
	}{
		{"*", "users:write", true}, {"*", "gateway:chat", true},
		{"*", "platform:tenants:write", false}, {"platform:*", "platform:tenants:write", false},
		{"platform:admin", "platform:admin", false}, {"*", "admin:write", false},
		{"roles:*", "roles:assign", true}, {"roles:assign", "roles:*", false},
		{"gateway:*", "users:write", false},
	} {
		if got := MatchScope(tc.held, tc.required); got != tc.want {
			t.Errorf("MatchScope(%q, %q) = %v", tc.held, tc.required, got)
		}
	}
}
