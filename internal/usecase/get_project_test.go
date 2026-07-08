package usecase_test

import (
	"context"
	"testing"

	"github.com/antoniojosev/traccia/internal/usecase"
)

func TestGetProject_FindsByID(t *testing.T) {
	projects := newFakeProjectRepo()
	create := usecase.NewCreateProject(projects, fakeKeyHasher{})
	getProject := usecase.NewGetProject(projects)

	created, _, err := create.Execute(context.Background(), "My Site", "example.com")
	if err != nil {
		t.Fatalf("unexpected error creating project: %v", err)
	}

	found, err := getProject.Execute(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found.ID != created.ID {
		t.Errorf("expected to find the created project, got a different one")
	}
}

func TestGetProject_ErrorsOnUnknownID(t *testing.T) {
	projects := newFakeProjectRepo()
	getProject := usecase.NewGetProject(projects)

	if _, err := getProject.Execute(context.Background(), "does-not-exist"); err == nil {
		t.Error("expected an error for an unknown project ID")
	}
}
