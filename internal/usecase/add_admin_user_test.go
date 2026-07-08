package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/antoniojosev/traccia/internal/usecase"
)

func TestAddAdminUser_WorksEvenWhenAccountsAlreadyExist(t *testing.T) {
	users := newFakeAdminUserRepo()
	register := usecase.NewRegisterAdminUser(users, fakePasswordHasher{})
	add := usecase.NewAddAdminUser(users, fakePasswordHasher{})

	if _, err := register.Execute(context.Background(), "antonio", "correct-horse-battery"); err != nil {
		t.Fatalf("unexpected error registering first account: %v", err)
	}

	user, err := add.Execute(context.Background(), "teammate", "another-long-password")
	if err != nil {
		t.Fatalf("expected AddAdminUser to work with an existing account present, got: %v", err)
	}
	if user.Username != "teammate" {
		t.Errorf("expected username teammate, got %q", user.Username)
	}
}

func TestAddAdminUser_RejectsDuplicateUsername(t *testing.T) {
	users := newFakeAdminUserRepo()
	add := usecase.NewAddAdminUser(users, fakePasswordHasher{})

	if _, err := add.Execute(context.Background(), "antonio", "correct-horse-battery"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err := add.Execute(context.Background(), "antonio", "a-different-password")
	if !errors.Is(err, usecase.ErrAdminUsernameTaken) {
		t.Errorf("expected ErrAdminUsernameTaken, got %v", err)
	}
}

func TestAddAdminUser_RejectsShortPassword(t *testing.T) {
	users := newFakeAdminUserRepo()
	add := usecase.NewAddAdminUser(users, fakePasswordHasher{})

	_, err := add.Execute(context.Background(), "antonio", "short")
	if !errors.Is(err, usecase.ErrAdminInvalidInput) {
		t.Errorf("expected ErrAdminInvalidInput, got %v", err)
	}
}
