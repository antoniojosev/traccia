//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/antoniojosev/traccia/internal/adapters/postgres"
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
