package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/antoniojosev/traccia/internal/usecase"
)

func TestIdentifyVisitor_RequiresProjectAndVisitor(t *testing.T) {
	uc := usecase.NewIdentifyVisitor(&fakeVisitorRepo{})

	err := uc.Execute(context.Background(), usecase.IdentifyVisitorInput{})

	if !errors.Is(err, usecase.ErrInvalidIdentify) {
		t.Fatalf("expected ErrInvalidIdentify, got %v", err)
	}
}

func TestIdentifyVisitor_UpsertsWithNameAndProperties(t *testing.T) {
	repo := &fakeVisitorRepo{}
	uc := usecase.NewIdentifyVisitor(repo)

	err := uc.Execute(context.Background(), usecase.IdentifyVisitorInput{
		ProjectID:  "p1",
		VisitorID:  "v1",
		Name:       "Antonio (yo mismo)",
		Properties: map[string]any{"plan": "pro"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.upserted) != 1 {
		t.Fatalf("expected 1 upsert, got %d", len(repo.upserted))
	}
	if repo.upserted[0].Name != "Antonio (yo mismo)" {
		t.Errorf("expected name to be passed through, got %q", repo.upserted[0].Name)
	}
}
