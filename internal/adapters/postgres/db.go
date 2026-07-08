// Package postgres is Traccia's default storage adapter, implementing the
// ports.EventRepository/ProjectRepository/VisitorRepository interfaces.
// Swap it for ClickHouse/SQLite by implementing the same ports.
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, databaseURL)
}
