package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/antoniojosev/traccia/internal/domain"
	"github.com/antoniojosev/traccia/internal/usecase"
)

type spySamplesRepo struct {
	fakeEventRepo
	lastFilter domain.StatsFilter
	lastType   domain.EventType
	lastName   string
	lastLimit  int
}

func (s *spySamplesRepo) RecentByName(_ context.Context, filter domain.StatsFilter, eventType domain.EventType, name string, limit int) ([]domain.EventDetail, error) {
	s.lastFilter = filter
	s.lastType = eventType
	s.lastName = name
	s.lastLimit = limit
	return []domain.EventDetail{{VisitorID: "v1"}}, nil
}

func TestGetEventSamples_PassesTypeAndNameThrough(t *testing.T) {
	repo := &spySamplesRepo{}
	uc := usecase.NewGetEventSamples(repo)

	samples, err := uc.Execute(context.Background(), usecase.GetEventSamplesInput{
		ProjectID: "p1",
		Type:      domain.EventTypeCustom,
		Name:      "calculator_used",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(samples) != 1 {
		t.Fatalf("expected 1 sample, got %d", len(samples))
	}
	if repo.lastType != domain.EventTypeCustom || repo.lastName != "calculator_used" {
		t.Errorf("expected type/name to be passed through, got %q/%q", repo.lastType, repo.lastName)
	}
}

func TestGetEventSamples_DefaultsLimit(t *testing.T) {
	repo := &spySamplesRepo{}
	uc := usecase.NewGetEventSamples(repo)

	_, err := uc.Execute(context.Background(), usecase.GetEventSamplesInput{ProjectID: "p1", Type: domain.EventTypeCustom, Name: "x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastLimit != 50 {
		t.Errorf("expected default limit 50, got %d", repo.lastLimit)
	}
}

func TestGetEventSamples_ClampsExcessiveLimit(t *testing.T) {
	repo := &spySamplesRepo{}
	uc := usecase.NewGetEventSamples(repo)

	_, err := uc.Execute(context.Background(), usecase.GetEventSamplesInput{ProjectID: "p1", Type: domain.EventTypeCustom, Name: "x", Limit: 10000})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastLimit != 50 {
		t.Errorf("expected clamped limit 50, got %d", repo.lastLimit)
	}
}

func TestGetEventSamples_DefaultsToLast7Days(t *testing.T) {
	repo := &spySamplesRepo{}
	uc := usecase.NewGetEventSamples(repo)

	_, err := uc.Execute(context.Background(), usecase.GetEventSamplesInput{ProjectID: "p1", Type: domain.EventTypeCustom, Name: "x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	span := repo.lastFilter.Until.Sub(repo.lastFilter.Since)
	if span < 6*24*time.Hour || span > 8*24*time.Hour {
		t.Errorf("expected roughly a 7 day span, got %v", span)
	}
}
