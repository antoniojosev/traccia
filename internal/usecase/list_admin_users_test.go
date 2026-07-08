package usecase_test

import (
	"context"
	"testing"

	"github.com/antoniojosev/traccia/internal/usecase"
)

func TestListAdminUsers_ReturnsAllCreatedAccounts(t *testing.T) {
	users := newFakeAdminUserRepo()
	add := usecase.NewAddAdminUser(users, fakePasswordHasher{})
	list := usecase.NewListAdminUsers(users)

	if _, err := add.Execute(context.Background(), "antonio", "correct-horse-battery"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := add.Execute(context.Background(), "teammate", "another-long-password"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := list.Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 admin users, got %d", len(result))
	}
}
