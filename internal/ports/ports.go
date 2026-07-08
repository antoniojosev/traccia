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
	// RecentByName drills into a single custom/error event name — e.g. the
	// dashboard's "calculator_used" panel showing the last N occurrences
	// with their full metadata, which a grouped aggregate can't express.
	RecentByName(ctx context.Context, filter domain.StatsFilter, eventType domain.EventType, name string, limit int) ([]domain.EventDetail, error)
	// MetadataBreakdown groups a single event name's occurrences by one
	// metadata key's value — e.g. "calculator_used" events grouped by
	// their "from_currency" value. This is what a plugin's registerPanel
	// groupBy actually computes.
	MetadataBreakdown(ctx context.Context, filter domain.StatsFilter, eventType domain.EventType, eventName, metadataKey string, limit int) ([]domain.NameCount, error)
}

// ProjectRepository manages tenants and their API keys.
type ProjectRepository interface {
	Create(ctx context.Context, project domain.Project) error
	FindByID(ctx context.Context, id string) (domain.Project, error)
	FindByAPIKeyHash(ctx context.Context, apiKeyHash string) (domain.Project, error)
	List(ctx context.Context) ([]domain.Project, error)
	Delete(ctx context.Context, id string) error
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

// AdminUserRepository stores the /admin panel's human accounts — separate
// from ProjectRepository, which is per-tenant API access.
type AdminUserRepository interface {
	Create(ctx context.Context, user domain.AdminUser) error
	FindByUsername(ctx context.Context, username string) (domain.AdminUser, error)
	Count(ctx context.Context) (int, error)
	List(ctx context.Context) ([]domain.AdminUser, error)
}

// PasswordHasher hashes and verifies human passwords. Deliberately a
// separate interface from APIKeyHasher: passwords are low-entropy and
// need a slow, salted algorithm (bcrypt); API keys are high-entropy random
// tokens where a fast hash (SHA-256) is the right tool. Picking one
// algorithm for both would be wrong in one direction or the other.
type PasswordHasher interface {
	Hash(plain string) (string, error)
	Verify(hash, plain string) bool
}
