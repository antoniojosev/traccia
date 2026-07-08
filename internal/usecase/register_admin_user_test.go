package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/antoniojosev/traccia/internal/usecase"
)

func TestRegisterAdminUser_CreatesFirstAccount(t *testing.T) {
	users := newFakeAdminUserRepo()
	uc := usecase.NewRegisterAdminUser(users, fakePasswordHasher{})

	user, err := uc.Execute(context.Background(), "antonio", "correct-horse-battery")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Username != "antonio" {
		t.Errorf("expected username antonio, got %q", user.Username)
	}
	if user.PasswordHash != "hashed:correct-horse-battery" {
		t.Errorf("expected password to be hashed, got %q", user.PasswordHash)
	}
}

func TestRegisterAdminUser_RefusesOnceAnAccountExists(t *testing.T) {
	users := newFakeAdminUserRepo()
	uc := usecase.NewRegisterAdminUser(users, fakePasswordHasher{})

	if _, err := uc.Execute(context.Background(), "antonio", "correct-horse-battery"); err != nil {
		t.Fatalf("unexpected error creating first account: %v", err)
	}

	_, err := uc.Execute(context.Background(), "someone-else", "another-password")
	if !errors.Is(err, usecase.ErrAdminSetupClosed) {
		t.Errorf("expected ErrAdminSetupClosed, got %v", err)
	}
}

func TestRegisterAdminUser_RejectsShortPassword(t *testing.T) {
	users := newFakeAdminUserRepo()
	uc := usecase.NewRegisterAdminUser(users, fakePasswordHasher{})

	_, err := uc.Execute(context.Background(), "antonio", "short")
	if !errors.Is(err, usecase.ErrAdminInvalidInput) {
		t.Errorf("expected ErrAdminInvalidInput, got %v", err)
	}
}

func TestRegisterAdminUser_RejectsShortUsername(t *testing.T) {
	users := newFakeAdminUserRepo()
	uc := usecase.NewRegisterAdminUser(users, fakePasswordHasher{})

	_, err := uc.Execute(context.Background(), "ab", "correct-horse-battery")
	if !errors.Is(err, usecase.ErrAdminInvalidInput) {
		t.Errorf("expected ErrAdminInvalidInput, got %v", err)
	}
}
