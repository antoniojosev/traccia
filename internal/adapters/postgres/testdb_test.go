//go:build integration

package postgres_test

import (
	"context"
	"os"
	"testing"

	"github.com/antoniojosev/traccia/internal/adapters/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

// migrationFiles must list every migration under migrations/, in order —
// there's no migration runner (see README), so tests apply the exact same
// files a fresh self-hosted instance would via docker-entrypoint-initdb.d.
var migrationFiles = []string{
	"../../../migrations/0001_init.sql",
	"../../../migrations/0002_plugin_kv.sql",
}

// setupTestPool connects to a throwaway Postgres instance and applies the
// real migration file (not a hand-maintained copy of it), so these tests
// exercise the exact SQL that ships. Point TRACCIA_TEST_DATABASE_URL at a
// disposable database — `make test-integration` starts one for you.
func setupTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	url := os.Getenv("TRACCIA_TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://traccia:traccia@localhost:5433/traccia_test?sslmode=disable"
	}

	ctx := context.Background()
	pool, err := postgres.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connecting to test postgres at %s: %v (run `make test-integration` to provision one)", url, err)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("pinging test postgres: %v (run `make test-integration` to provision one)", err)
	}

	resetSchema(t, ctx, pool)
	t.Cleanup(pool.Close)
	return pool
}

func resetSchema(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	if _, err := pool.Exec(ctx, `DROP TABLE IF EXISTS events, visitors, projects, plugin_kv CASCADE`); err != nil {
		t.Fatalf("dropping tables: %v", err)
	}

	for _, path := range migrationFiles {
		migration, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading migration file %s: %v", path, err)
		}
		if _, err := pool.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("applying migration %s: %v", path, err)
		}
	}
}
