package usecase_test

import (
	"context"
	"testing"

	"github.com/antoniojosev/traccia/internal/usecase"
)

func TestUpdateProject_RenamesAndChangesDomain(t *testing.T) {
	projects := newFakeProjectRepo()
	createUC := usecase.NewCreateProject(projects, fakeKeyHasher{})
	updateUC := usecase.NewUpdateProject(projects)

	created, _, err := createUC.Execute(context.Background(), "My Site", "example.com")
	if err != nil {
		t.Fatalf("unexpected error creating project: %v", err)
	}

	updated, err := updateUC.Execute(context.Background(), created.ID, "New Name", "new.example.com")
	if err != nil {
		t.Fatalf("unexpected error updating project: %v", err)
	}
	if updated.Name != "New Name" || updated.Domain != "new.example.com" {
		t.Errorf("expected updated fields, got %+v", updated)
	}
	if updated.APIKeyHash != created.APIKeyHash {
		t.Errorf("renaming should not change the API key hash")
	}

	found, err := projects.FindByID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("unexpected error fetching project: %v", err)
	}
	if found.Name != "New Name" {
		t.Errorf("expected persisted name to be updated, got %q", found.Name)
	}
}

func TestUpdateProject_RejectsEmptyName(t *testing.T) {
	projects := newFakeProjectRepo()
	updateUC := usecase.NewUpdateProject(projects)

	if _, err := updateUC.Execute(context.Background(), "some-id", "", "example.com"); err != usecase.ErrProjectNameRequired {
		t.Fatalf("expected ErrProjectNameRequired, got %v", err)
	}
}

func TestUpdateProject_UnknownProjectReturnsError(t *testing.T) {
	projects := newFakeProjectRepo()
	updateUC := usecase.NewUpdateProject(projects)

	if _, err := updateUC.Execute(context.Background(), "missing", "New Name", "example.com"); err == nil {
		t.Fatal("expected an error for an unknown project")
	}
}
