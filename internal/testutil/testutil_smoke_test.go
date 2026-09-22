package testutil_test

import (
	"testing"

	"github.com/Abraxas-365/freerouter/internal/testutil"
)

// TestPostgresDB_Smoke verifies the fixture starts a container, applies
// every migration, and returns a working connection.
func TestPostgresDB_Smoke(t *testing.T) {
	db := testutil.PostgresDB(t)

	var count int
	if err := db.Get(&count, `SELECT COUNT(*) FROM providers`); err != nil {
		t.Fatalf("providers table not migrated: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected empty providers table, got %d rows", count)
	}

	// Spot-check a later migration ran too.
	if _, err := db.Exec(`SELECT 1 FROM webhook_configs LIMIT 1`); err != nil {
		t.Fatalf("webhook_configs table not migrated: %v", err)
	}
}

// TestRedisClient_Smoke verifies the fixture starts a container and
// returns a working connection.
func TestRedisClient_Smoke(t *testing.T) {
	client := testutil.RedisClient(t)
	ctx := t.Context()

	if err := client.Set(ctx, "k", "v", 0).Err(); err != nil {
		t.Fatalf("failed to set key: %v", err)
	}
	val, err := client.Get(ctx, "k").Result()
	if err != nil {
		t.Fatalf("failed to get key: %v", err)
	}
	if val != "v" {
		t.Fatalf("expected 'v', got %q", val)
	}
}
