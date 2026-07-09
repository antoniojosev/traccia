package usecase_test

import (
	"context"
	"testing"

	"github.com/antoniojosev/traccia/internal/usecase"
)

func TestGetAdminUser_FindsByID(t *testing.T) {
	users := newFakeAdminUserRepo()
	register := usecase.NewRegisterAdminUser(users, fakePasswordHasher{})
	get := usecase.NewGetAdminUser(users)

	created, err := register.Execute(context.Background(), "antonio", "correct-horse-battery")
	if err != nil {
		t.Fatalf("unexpected error registering account: %v", err)
	}

	found, err := get.Execute(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found.Username != "antonio" {
		t.Errorf("expected username antonio, got %q", found.Username)
	}
}

func TestGetAdminUser_UnknownIDReturnsError(t *testing.T) {
	users := newFakeAdminUserRepo()
	get := usecase.NewGetAdminUser(users)

	if _, err := get.Execute(context.Background(), "missing"); err == nil {
		t.Fatal("expected an error for an unknown ID")
	}
}
