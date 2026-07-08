//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/antoniojosev/traccia/internal/adapters/postgres"
)

func TestPluginKVRepository_SetAndGet(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	kv := postgres.NewPluginKVRepository(pool)

	if err := kv.Set(ctx, "counter-plugin", "count", "1"); err != nil {
		t.Fatalf("set: %v", err)
	}

	value, ok, err := kv.Get(ctx, "counter-plugin", "count")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !ok || value != "1" {
		t.Errorf("expected value=1 ok=true, got value=%q ok=%v", value, ok)
	}
}

func TestPluginKVRepository_GetMissingKeyReturnsNotOK(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	kv := postgres.NewPluginKVRepository(pool)

	_, ok, err := kv.Get(ctx, "some-plugin", "does-not-exist")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected ok=false for a missing key")
	}
}

func TestPluginKVRepository_SetOverwritesAndIsNamespacedPerPlugin(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	kv := postgres.NewPluginKVRepository(pool)

	if err := kv.Set(ctx, "plugin-a", "key", "from-a"); err != nil {
		t.Fatalf("set plugin-a: %v", err)
	}
	if err := kv.Set(ctx, "plugin-b", "key", "from-b"); err != nil {
		t.Fatalf("set plugin-b: %v", err)
	}
	if err := kv.Set(ctx, "plugin-a", "key", "from-a-updated"); err != nil {
		t.Fatalf("overwrite plugin-a: %v", err)
	}

	valueA, _, err := kv.Get(ctx, "plugin-a", "key")
	if err != nil {
		t.Fatalf("get plugin-a: %v", err)
	}
	valueB, _, err := kv.Get(ctx, "plugin-b", "key")
	if err != nil {
		t.Fatalf("get plugin-b: %v", err)
	}

	if valueA != "from-a-updated" {
		t.Errorf("expected overwritten value for plugin-a, got %q", valueA)
	}
	if valueB != "from-b" {
		t.Errorf("expected plugin-b's value to be untouched by plugin-a's writes, got %q", valueB)
	}
}
