package usecase_test

import (
	"context"
	"testing"

	"github.com/antoniojosev/traccia/internal/usecase"
)

func TestListProjects_ReturnsAllCreatedProjects(t *testing.T) {
	projects := newFakeProjectRepo()
	create := usecase.NewCreateProject(projects, fakeKeyHasher{})
	list := usecase.NewListProjects(projects)

	if _, _, err := create.Execute(context.Background(), "Site A", "a.example.com"); err != nil {
		t.Fatalf("creating site A: %v", err)
	}
	if _, _, err := create.Execute(context.Background(), "Site B", "b.example.com"); err != nil {
		t.Fatalf("creating site B: %v", err)
	}

	result, err := list.Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(result))
	}
}

func TestListProjects_EmptyWhenNoneCreated(t *testing.T) {
	projects := newFakeProjectRepo()
	list := usecase.NewListProjects(projects)

	result, err := list.Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected no projects, got %d", len(result))
	}
}
