package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/antoniojosev/traccia/internal/usecase"
)

func TestDeleteAdminUser_RemovesAccountWhenOthersRemain(t *testing.T) {
	users := newFakeAdminUserRepo()
	register := usecase.NewRegisterAdminUser(users, fakePasswordHasher{})
	add := usecase.NewAddAdminUser(users, fakePasswordHasher{})
	deleteUC := usecase.NewDeleteAdminUser(users)

	first, err := register.Execute(context.Background(), "antonio", "correct-horse-battery")
	if err != nil {
		t.Fatalf("unexpected error registering first account: %v", err)
	}
	second, err := add.Execute(context.Background(), "teammate", "another-long-password")
	if err != nil {
		t.Fatalf("unexpected error adding second account: %v", err)
	}

	if err := deleteUC.Execute(context.Background(), second.ID); err != nil {
		t.Fatalf("unexpected error deleting second account: %v", err)
	}

	if _, err := users.FindByID(context.Background(), first.ID); err != nil {
		t.Errorf("expected first account to remain, got error: %v", err)
	}
	if _, err := users.FindByID(context.Background(), second.ID); err == nil {
		t.Errorf("expected second account to be gone")
	}
}

func TestDeleteAdminUser_RefusesToDeleteLastAdmin(t *testing.T) {
	users := newFakeAdminUserRepo()
	register := usecase.NewRegisterAdminUser(users, fakePasswordHasher{})
	deleteUC := usecase.NewDeleteAdminUser(users)

	only, err := register.Execute(context.Background(), "antonio", "correct-horse-battery")
	if err != nil {
		t.Fatalf("unexpected error registering account: %v", err)
	}

	err = deleteUC.Execute(context.Background(), only.ID)
	if !errors.Is(err, usecase.ErrCannotDeleteLastAdmin) {
		t.Errorf("expected ErrCannotDeleteLastAdmin, got %v", err)
	}
}
