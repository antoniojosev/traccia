//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/antoniojosev/traccia/internal/adapters/postgres"
	"github.com/antoniojosev/traccia/internal/domain"
	"github.com/google/uuid"
)

func TestProjectRepository_CreateAndFind(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	projects := postgres.NewProjectRepository(pool)

	project := createTestProject(t, ctx, projects)

	found, err := projects.FindByID(ctx, project.ID)
	if err != nil {
		t.Fatalf("finding by id: %v", err)
	}
	if found.Name != project.Name {
		t.Errorf("expected name %q, got %q", project.Name, found.Name)
	}

	foundByHash, err := projects.FindByAPIKeyHash(ctx, project.APIKeyHash)
	if err != nil {
		t.Fatalf("finding by hash: %v", err)
	}
	if foundByHash.ID != project.ID {
		t.Errorf("expected to find the same project by hash")
	}

	if _, err := projects.FindByAPIKeyHash(ctx, "does-not-exist"); !errors.Is(err, postgres.ErrProjectNotFound) {
		t.Errorf("expected ErrProjectNotFound for unknown hash, got %v", err)
	}
	if _, err := projects.FindByID(ctx, "00000000-0000-0000-0000-000000000000"); !errors.Is(err, postgres.ErrProjectNotFound) {
		t.Errorf("expected ErrProjectNotFound for unknown id, got %v", err)
	}
}

func TestProjectRepository_ListReturnsAllNewestFirst(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	projects := postgres.NewProjectRepository(pool)

	first := createTestProject(t, ctx, projects)
	time.Sleep(10 * time.Millisecond) // created_at has second-ish precision concerns aside, force distinct ordering
	second := createTestProject(t, ctx, projects)

	list, err := projects.List(ctx)
	if err != nil {
		t.Fatalf("listing projects: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(list))
	}
	if list[0].ID != second.ID || list[1].ID != first.ID {
		t.Errorf("expected newest-first order, got %+v", list)
	}
}

func TestProjectRepository_DeleteCascadesToEvents(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	projects := postgres.NewProjectRepository(pool)
	events := postgres.NewEventRepository(pool)

	project := createTestProject(t, ctx, projects)
	if err := events.Save(ctx, domain.Event{
		ProjectID: project.ID, VisitorID: uuid.NewString(), Type: domain.EventTypePageview, CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("saving event: %v", err)
	}

	if err := projects.Delete(ctx, project.ID); err != nil {
		t.Fatalf("deleting project: %v", err)
	}

	if _, err := projects.FindByID(ctx, project.ID); !errors.Is(err, postgres.ErrProjectNotFound) {
		t.Errorf("expected ErrProjectNotFound after delete, got %v", err)
	}

	var eventCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM events WHERE project_id = $1`, project.ID).Scan(&eventCount); err != nil {
		t.Fatalf("counting events: %v", err)
	}
	if eventCount != 0 {
		t.Errorf("expected the FK cascade to delete the project's events too, got %d remaining", eventCount)
	}
}

func TestProjectRepository_DeleteIsANoOpForUnknownID(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	projects := postgres.NewProjectRepository(pool)

	if err := projects.Delete(ctx, "00000000-0000-0000-0000-000000000000"); err != nil {
		t.Errorf("expected deleting an unknown id to be a no-op, got error: %v", err)
	}
}
