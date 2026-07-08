package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/antoniojosev/traccia/internal/usecase"
)

func TestAuthenticateAdminUser_AcceptsCorrectCredentials(t *testing.T) {
	users := newFakeAdminUserRepo()
	register := usecase.NewRegisterAdminUser(users, fakePasswordHasher{})
	authenticate := usecase.NewAuthenticateAdminUser(users, fakePasswordHasher{})

	if _, err := register.Execute(context.Background(), "antonio", "correct-horse-battery"); err != nil {
		t.Fatalf("unexpected error registering: %v", err)
	}

	user, err := authenticate.Execute(context.Background(), "antonio", "correct-horse-battery")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Username != "antonio" {
		t.Errorf("expected antonio, got %q", user.Username)
	}
}

func TestAuthenticateAdminUser_RejectsWrongPassword(t *testing.T) {
	users := newFakeAdminUserRepo()
	register := usecase.NewRegisterAdminUser(users, fakePasswordHasher{})
	authenticate := usecase.NewAuthenticateAdminUser(users, fakePasswordHasher{})

	if _, err := register.Execute(context.Background(), "antonio", "correct-horse-battery"); err != nil {
		t.Fatalf("unexpected error registering: %v", err)
	}

	if _, err := authenticate.Execute(context.Background(), "antonio", "wrong-password"); !errors.Is(err, usecase.ErrAdminInvalidCredentials) {
		t.Errorf("expected ErrAdminInvalidCredentials, got %v", err)
	}
}

func TestAuthenticateAdminUser_RejectsUnknownUsername(t *testing.T) {
	users := newFakeAdminUserRepo()
	authenticate := usecase.NewAuthenticateAdminUser(users, fakePasswordHasher{})

	if _, err := authenticate.Execute(context.Background(), "nobody", "whatever"); !errors.Is(err, usecase.ErrAdminInvalidCredentials) {
		t.Errorf("expected ErrAdminInvalidCredentials, got %v", err)
	}
}
