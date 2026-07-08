package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/antoniojosev/traccia/internal/domain"
	"github.com/antoniojosev/traccia/internal/usecase"
)

type spyEventRepo struct {
	fakeEventRepo
	lastFilter domain.StatsFilter
}

func (s *spyEventRepo) Stats(ctx context.Context, filter domain.StatsFilter) (domain.Stats, error) {
	s.lastFilter = filter
	return domain.Stats{}, nil
}

func TestGetStats_DefaultsToLast7DaysWhenUnset(t *testing.T) {
	repo := &spyEventRepo{}
	uc := usecase.NewGetStats(repo)

	_, err := uc.Execute(context.Background(), usecase.GetStatsInput{ProjectID: "p1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	span := repo.lastFilter.Until.Sub(repo.lastFilter.Since)
	if span < 6*24*time.Hour || span > 8*24*time.Hour {
		t.Errorf("expected roughly a 7 day span, got %v", span)
	}
}

func TestGetStats_PassesExcludeNamedThrough(t *testing.T) {
	repo := &spyEventRepo{}
	uc := usecase.NewGetStats(repo)

	_, err := uc.Execute(context.Background(), usecase.GetStatsInput{ProjectID: "p1", ExcludeNamed: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.lastFilter.ExcludeNamed {
		t.Errorf("expected ExcludeNamed to be passed through to the filter")
	}
}

func TestGetStats_PassesIncludeBotsThrough(t *testing.T) {
	repo := &spyEventRepo{}
	uc := usecase.NewGetStats(repo)

	_, err := uc.Execute(context.Background(), usecase.GetStatsInput{ProjectID: "p1", IncludeBots: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.lastFilter.IncludeBots {
		t.Errorf("expected IncludeBots to be passed through to the filter")
	}
}
