package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PluginKVRepository implements plugins.KVStore. It lives in this package
// (rather than importing the plugins package, which would create a cycle
// since plugins wires this in) — Go interfaces are structural, so no
// import is needed for it to satisfy plugins.KVStore.
type PluginKVRepository struct {
	pool *pgxpool.Pool
}

func NewPluginKVRepository(pool *pgxpool.Pool) *PluginKVRepository {
	return &PluginKVRepository{pool: pool}
}

func (r *PluginKVRepository) Get(ctx context.Context, plugin, key string) (string, bool, error) {
	var value string
	err := r.pool.QueryRow(ctx, `SELECT value FROM plugin_kv WHERE plugin = $1 AND key = $2`, plugin, key).Scan(&value)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func (r *PluginKVRepository) Set(ctx context.Context, plugin, key, value string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO plugin_kv (plugin, key, value, updated_at) VALUES ($1, $2, $3, now())
		ON CONFLICT (plugin, key) DO UPDATE SET value = EXCLUDED.value, updated_at = EXCLUDED.updated_at
	`, plugin, key, value)
	return err
}
