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

func TestAdminUserRepository_CreateAndFindByUsername(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	users := postgres.NewAdminUserRepository(pool)

	user := domain.AdminUser{
		ID:           uuid.NewString(),
		Username:     "antonio",
		PasswordHash: "bcrypt-hash-placeholder",
		CreatedAt:    time.Now().UTC(),
	}
	if err := users.Create(ctx, user); err != nil {
		t.Fatalf("creating admin user: %v", err)
	}

	found, err := users.FindByUsername(ctx, "antonio")
	if err != nil {
		t.Fatalf("finding by username: %v", err)
	}
	if found.ID != user.ID || found.PasswordHash != user.PasswordHash {
		t.Errorf("expected to find the created user, got %+v", found)
	}

	if _, err := users.FindByUsername(ctx, "does-not-exist"); !errors.Is(err, postgres.ErrAdminUserNotFound) {
		t.Errorf("expected ErrAdminUserNotFound, got %v", err)
	}
}

func TestAdminUserRepository_Count(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	users := postgres.NewAdminUserRepository(pool)

	count, err := users.Count(ctx)
	if err != nil {
		t.Fatalf("counting: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 users on a fresh schema, got %d", count)
	}

	if err := users.Create(ctx, domain.AdminUser{
		ID: uuid.NewString(), Username: "antonio", PasswordHash: "x", CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("creating: %v", err)
	}

	count, err = users.Count(ctx)
	if err != nil {
		t.Fatalf("counting: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 user after creating one, got %d", count)
	}
}

func TestAdminUserRepository_UsernameIsUnique(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	users := postgres.NewAdminUserRepository(pool)

	first := domain.AdminUser{ID: uuid.NewString(), Username: "antonio", PasswordHash: "x", CreatedAt: time.Now().UTC()}
	if err := users.Create(ctx, first); err != nil {
		t.Fatalf("creating first user: %v", err)
	}

	second := domain.AdminUser{ID: uuid.NewString(), Username: "antonio", PasswordHash: "y", CreatedAt: time.Now().UTC()}
	if err := users.Create(ctx, second); err == nil {
		t.Error("expected a unique constraint violation for a duplicate username")
	}
}
