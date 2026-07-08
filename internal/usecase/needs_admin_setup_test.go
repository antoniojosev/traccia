package usecase_test

import (
	"context"
	"testing"

	"github.com/antoniojosev/traccia/internal/usecase"
)

func TestNeedsAdminSetup_TrueWhenNoUsers(t *testing.T) {
	users := newFakeAdminUserRepo()
	uc := usecase.NewNeedsAdminSetup(users)

	needsSetup, err := uc.Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !needsSetup {
		t.Error("expected setup to be needed with zero users")
	}
}

func TestNeedsAdminSetup_FalseOnceAUserExists(t *testing.T) {
	users := newFakeAdminUserRepo()
	register := usecase.NewRegisterAdminUser(users, fakePasswordHasher{})
	uc := usecase.NewNeedsAdminSetup(users)

	if _, err := register.Execute(context.Background(), "antonio", "correct-horse-battery"); err != nil {
		t.Fatalf("unexpected error registering: %v", err)
	}

	needsSetup, err := uc.Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if needsSetup {
		t.Error("expected setup to be closed once a user exists")
	}
}
