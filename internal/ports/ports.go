package ports

import (
	"context"

	"github.com/antoniojosev/traccia/internal/domain"
)

// EventRepository persists and queries events. Implement this against any
// store (Postgres ships by default; ClickHouse/SQLite are drop-in swaps).
type EventRepository interface {
	Save(ctx context.Context, event domain.Event) error
	Stats(ctx context.Context, filter domain.StatsFilter) (domain.Stats, error)
}

// ProjectRepository manages tenants and their API keys.
type ProjectRepository interface {
	Create(ctx context.Context, project domain.Project) error
	FindByID(ctx context.Context, id string) (domain.Project, error)
	FindByAPIKeyHash(ctx context.Context, apiKeyHash string) (domain.Project, error)
}

// VisitorRepository stores the durable identity set via Identify, keyed by
// (project_id, visitor_id) — separate from the event log so traits don't
// need to be repeated on every tracked event.
type VisitorRepository interface {
	Upsert(ctx context.Context, visitor domain.Visitor) error
}

// UserAgentParser turns a raw User-Agent header into structured device info.
// The default adapter is a lightweight heuristic parser; swap it for a
// fuller library (e.g. one backed by the ua-parser regex database) without
// touching any usecase.
type UserAgentParser interface {
	Parse(userAgent string) domain.DeviceInfo
}

// GeoResolver turns an IP into coarse geo info. The default adapter is a
// no-op; plug in a MaxMind/IP2Location adapter to enable it.
type GeoResolver interface {
	Resolve(ip string) domain.GeoInfo
}

// APIKeyHasher hashes and verifies API keys, and mints new ones.
type APIKeyHasher interface {
	Generate() (plainKey string, hash string, err error)
	Hash(plainKey string) string
}
