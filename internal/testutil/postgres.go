// Package testutil provides shared testcontainers-backed PostgreSQL and
// Redis fixtures for integration/e2e tests. It is test-only infrastructure
// and does not participate in the hexagonal module dependency graph.
package testutil

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// PostgresDB starts a disposable PostgreSQL 16 container, applies every
// migration in migrations/*.up.sql (in filename order), and returns a
// connected *sqlx.DB. The container and connection are torn down
// automatically via t.Cleanup.
func PostgresDB(t *testing.T) *sqlx.DB {
	t.Helper()
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("freerouter_test"),
		tcpostgres.WithUsername("freerouter"),
		tcpostgres.WithPassword("freerouter"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2),
		),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Logf("failed to terminate postgres container: %v", err)
		}
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get postgres connection string: %v", err)
	}

	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	applyMigrations(t, db)

	return db
}

// applyMigrations runs every migrations/*.up.sql file (repo root, sibling
// to internal/) against db, in filename order.
func applyMigrations(t *testing.T, db *sqlx.DB) {
	t.Helper()

	dir := migrationsDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read migrations dir %s: %v", dir, err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".sql" {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, name := range files {
		path := filepath.Join(dir, name)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read migration %s: %v", path, err)
		}
		if _, err := db.Exec(string(content)); err != nil {
			t.Fatalf("failed to apply migration %s: %v", name, err)
		}
	}
}

// migrationsDir resolves the repository's migrations/ directory relative
// to this source file, so it works regardless of the test package's cwd.
func migrationsDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve testutil source path")
	}
	// this file: <root>/internal/testutil/postgres.go
	root := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
	return filepath.Join(root, "migrations")
}
