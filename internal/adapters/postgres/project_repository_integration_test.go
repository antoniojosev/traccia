//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"

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
