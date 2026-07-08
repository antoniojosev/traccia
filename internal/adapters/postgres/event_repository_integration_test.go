//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/antoniojosev/traccia/internal/adapters/postgres"
	"github.com/antoniojosev/traccia/internal/domain"
	"github.com/google/uuid"
)

func createTestProject(t *testing.T, ctx context.Context, projects *postgres.ProjectRepository) domain.Project {
	t.Helper()
	project := domain.Project{
		ID:         uuid.NewString(),
		Name:       "Test Project",
		Domain:     "example.com",
		APIKeyHash: "hash-" + uuid.NewString(),
		CreatedAt:  time.Now().UTC(),
	}
	if err := projects.Create(ctx, project); err != nil {
		t.Fatalf("creating test project: %v", err)
	}
	return project
}

func TestEventRepository_SaveAndStats(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	projects := postgres.NewProjectRepository(pool)
	events := postgres.NewEventRepository(pool)

	project := createTestProject(t, ctx, projects)
	now := time.Now().UTC()
	visitorA := uuid.NewString()
	visitorB := uuid.NewString()

	mustSave := func(e domain.Event) {
		t.Helper()
		if err := events.Save(ctx, e); err != nil {
			t.Fatalf("saving event: %v", err)
		}
	}

	mustSave(domain.Event{
		ProjectID: project.ID, VisitorID: visitorA, Type: domain.EventTypePageview,
		Path: "/", Referrer: "https://google.com", CreatedAt: now,
	})
	mustSave(domain.Event{
		ProjectID: project.ID, VisitorID: visitorA, Type: domain.EventTypePageview,
		Path: "/pricing", CreatedAt: now,
	})
	mustSave(domain.Event{
		ProjectID: project.ID, VisitorID: visitorB, Type: domain.EventTypeCustom, Name: "signup_clicked",
		Path: "/", Metadata: map[string]any{"plan": "pro"}, CreatedAt: now,
	})

	stats, err := events.Stats(ctx, domain.StatsFilter{
		ProjectID: project.ID,
		Since:     now.Add(-time.Hour),
		Until:     now.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("querying stats: %v", err)
	}

	if stats.TotalEvents != 3 {
		t.Errorf("expected 3 total events, got %d", stats.TotalEvents)
	}
	if stats.UniqueVisitors != 2 {
		t.Errorf("expected 2 unique visitors, got %d", stats.UniqueVisitors)
	}
	if len(stats.TopPaths) == 0 || stats.TopPaths[0].Path != "/" || stats.TopPaths[0].Count != 2 {
		t.Errorf("expected top path '/' with count 2, got %+v", stats.TopPaths)
	}
	if len(stats.TopReferrers) != 1 || stats.TopReferrers[0].Referrer != "https://google.com" {
		t.Errorf("expected 1 referrer, got %+v", stats.TopReferrers)
	}
	if len(stats.VisitsOverTime) != 1 || stats.VisitsOverTime[0].Count != 3 {
		t.Errorf("expected 1 bucket with 3 events, got %+v", stats.VisitsOverTime)
	}
}

func TestEventRepository_Stats_ExcludesBotsByDefault(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	projects := postgres.NewProjectRepository(pool)
	events := postgres.NewEventRepository(pool)

	project := createTestProject(t, ctx, projects)
	now := time.Now().UTC()

	must := func(e domain.Event) {
		t.Helper()
		if err := events.Save(ctx, e); err != nil {
			t.Fatalf("saving event: %v", err)
		}
	}

	must(domain.Event{ProjectID: project.ID, VisitorID: uuid.NewString(), Type: domain.EventTypePageview, Device: domain.DeviceInfo{DeviceType: "desktop"}, CreatedAt: now})
	must(domain.Event{ProjectID: project.ID, VisitorID: uuid.NewString(), Type: domain.EventTypePageview, Device: domain.DeviceInfo{DeviceType: "bot"}, CreatedAt: now})

	filter := domain.StatsFilter{ProjectID: project.ID, Since: now.Add(-time.Hour), Until: now.Add(time.Hour)}

	withoutBots, err := events.Stats(ctx, filter)
	if err != nil {
		t.Fatalf("querying stats: %v", err)
	}
	if withoutBots.TotalEvents != 1 {
		t.Errorf("expected bots excluded by default, got %d total events", withoutBots.TotalEvents)
	}

	filter.IncludeBots = true
	withBots, err := events.Stats(ctx, filter)
	if err != nil {
		t.Fatalf("querying stats with bots included: %v", err)
	}
	if withBots.TotalEvents != 2 {
		t.Errorf("expected 2 total events with bots included, got %d", withBots.TotalEvents)
	}
}

func TestEventRepository_Stats_ExcludeNamedVisitor(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	projects := postgres.NewProjectRepository(pool)
	events := postgres.NewEventRepository(pool)
	visitors := postgres.NewVisitorRepository(pool)

	project := createTestProject(t, ctx, projects)
	now := time.Now().UTC()
	ownerVisitorID := uuid.NewString()
	realVisitorID := uuid.NewString()

	if err := events.Save(ctx, domain.Event{ProjectID: project.ID, VisitorID: ownerVisitorID, Type: domain.EventTypePageview, CreatedAt: now}); err != nil {
		t.Fatalf("saving owner event: %v", err)
	}
	if err := events.Save(ctx, domain.Event{ProjectID: project.ID, VisitorID: realVisitorID, Type: domain.EventTypePageview, CreatedAt: now}); err != nil {
		t.Fatalf("saving real event: %v", err)
	}
	if err := visitors.Upsert(ctx, domain.Visitor{ProjectID: project.ID, VisitorID: ownerVisitorID, Name: "Antonio (yo mismo)", UpdatedAt: now}); err != nil {
		t.Fatalf("identifying owner visitor: %v", err)
	}

	filter := domain.StatsFilter{ProjectID: project.ID, Since: now.Add(-time.Hour), Until: now.Add(time.Hour), ExcludeNamed: true}
	stats, err := events.Stats(ctx, filter)
	if err != nil {
		t.Fatalf("querying stats: %v", err)
	}
	if stats.TotalEvents != 1 {
		t.Errorf("expected owner's event excluded, got %d total events", stats.TotalEvents)
	}
}

// TestEventRepository_Save_RejectsUnknownProject verifies the assumption
// the HTTP layer relies on: the events.project_id foreign key rejects
// unknown project IDs, so /api/v1/track doesn't need its own repository
// lookup on every request (see ingest_handler.go's comment).
func TestEventRepository_Save_RejectsUnknownProject(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	events := postgres.NewEventRepository(pool)

	err := events.Save(ctx, domain.Event{
		ProjectID: uuid.NewString(),
		VisitorID: uuid.NewString(),
		Type:      domain.EventTypePageview,
		CreatedAt: time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("expected foreign key violation for unknown project_id, got nil error")
	}
}

func TestEventRepository_Stats_BreakdownsAndDrilldown(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	projects := postgres.NewProjectRepository(pool)
	events := postgres.NewEventRepository(pool)

	project := createTestProject(t, ctx, projects)
	now := time.Now().UTC()

	must := func(e domain.Event) {
		t.Helper()
		if err := events.Save(ctx, e); err != nil {
			t.Fatalf("saving event: %v", err)
		}
	}

	must(domain.Event{
		ProjectID: project.ID, VisitorID: uuid.NewString(), Type: domain.EventTypePageview,
		Device: domain.DeviceInfo{DeviceType: "desktop", Browser: "chrome", OS: "linux"}, CreatedAt: now,
	})
	must(domain.Event{
		ProjectID: project.ID, VisitorID: uuid.NewString(), Type: domain.EventTypeCustom, Name: "calculator_used",
		Metadata: map[string]any{"amount": 100}, CreatedAt: now,
	})
	must(domain.Event{
		ProjectID: project.ID, VisitorID: uuid.NewString(), Type: domain.EventTypeCustom, Name: "calculator_used",
		Metadata: map[string]any{"amount": 250}, CreatedAt: now,
	})
	must(domain.Event{
		ProjectID: project.ID, VisitorID: uuid.NewString(), Type: domain.EventTypeError, Name: "unhandled_exception",
		CreatedAt: now,
	})

	filter := domain.StatsFilter{ProjectID: project.ID, Since: now.Add(-time.Hour), Until: now.Add(time.Hour)}
	stats, err := events.Stats(ctx, filter)
	if err != nil {
		t.Fatalf("querying stats: %v", err)
	}

	if len(stats.DeviceTypes) != 1 || stats.DeviceTypes[0].Name != "desktop" || stats.DeviceTypes[0].Count != 1 {
		t.Errorf("expected 1 desktop device, got %+v", stats.DeviceTypes)
	}
	if len(stats.CustomEventNames) != 1 || stats.CustomEventNames[0].Name != "calculator_used" || stats.CustomEventNames[0].Count != 2 {
		t.Errorf("expected calculator_used x2, got %+v", stats.CustomEventNames)
	}
	if len(stats.ErrorEventNames) != 1 || stats.ErrorEventNames[0].Name != "unhandled_exception" {
		t.Errorf("expected 1 error event name, got %+v", stats.ErrorEventNames)
	}

	samples, err := events.RecentByName(ctx, filter, domain.EventTypeCustom, "calculator_used", 10)
	if err != nil {
		t.Fatalf("querying recent by name: %v", err)
	}
	if len(samples) != 2 {
		t.Fatalf("expected 2 samples, got %d", len(samples))
	}
	amounts := map[float64]bool{}
	for _, s := range samples {
		if amount, ok := s.Metadata["amount"].(float64); ok {
			amounts[amount] = true
		}
	}
	if !amounts[100] || !amounts[250] {
		t.Errorf("expected both metadata amounts present, got samples %+v", samples)
	}
}
