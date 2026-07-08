package usecase_test

import (
	"context"
	"testing"

	"github.com/antoniojosev/traccia/internal/usecase"
)

func TestCreateProject_PersistsHashedKeyAndReturnsPlainOnce(t *testing.T) {
	projects := newFakeProjectRepo()
	uc := usecase.NewCreateProject(projects, fakeKeyHasher{})

	project, plainKey, err := uc.Execute(context.Background(), "My Site", "example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plainKey != "plain-key" {
		t.Errorf("expected plain key from hasher, got %q", plainKey)
	}
	if project.APIKeyHash != "hash-of-plain-key" {
		t.Errorf("expected stored project to hold the hash, not the plain key, got %q", project.APIKeyHash)
	}
}

func TestAuthenticateProject_LooksUpByHashedKey(t *testing.T) {
	projects := newFakeProjectRepo()
	createUC := usecase.NewCreateProject(projects, fakeKeyHasher{})
	authUC := usecase.NewAuthenticateProject(projects, fakeKeyHasher{})

	created, _, err := createUC.Execute(context.Background(), "My Site", "example.com")
	if err != nil {
		t.Fatalf("unexpected error creating project: %v", err)
	}

	found, err := authUC.Execute(context.Background(), "plain-key")
	if err != nil {
		t.Fatalf("unexpected error authenticating: %v", err)
	}
	if found.ID != created.ID {
		t.Errorf("expected to find the created project, got a different one")
	}
}
