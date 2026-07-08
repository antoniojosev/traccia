package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/antoniojosev/traccia/internal/domain"
	"github.com/antoniojosev/traccia/internal/usecase"
)

type spyMetadataBreakdownRepo struct {
	fakeEventRepo
	lastFilter      domain.StatsFilter
	lastType        domain.EventType
	lastEventName   string
	lastMetadataKey string
	lastLimit       int
}

func (s *spyMetadataBreakdownRepo) MetadataBreakdown(_ context.Context, filter domain.StatsFilter, eventType domain.EventType, eventName, metadataKey string, limit int) ([]domain.NameCount, error) {
	s.lastFilter = filter
	s.lastType = eventType
	s.lastEventName = eventName
	s.lastMetadataKey = metadataKey
	s.lastLimit = limit
	return []domain.NameCount{{Name: "USD", Count: 5}}, nil
}

func TestGetMetadataBreakdown_PassesArgumentsThrough(t *testing.T) {
	repo := &spyMetadataBreakdownRepo{}
	uc := usecase.NewGetMetadataBreakdown(repo)

	result, err := uc.Execute(context.Background(), usecase.GetMetadataBreakdownInput{
		ProjectID:   "p1",
		Type:        domain.EventTypeCustom,
		EventName:   "calculator_used",
		MetadataKey: "from_currency",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 || result[0].Name != "USD" {
		t.Errorf("expected the repo's result to pass through, got %+v", result)
	}
	if repo.lastType != domain.EventTypeCustom || repo.lastEventName != "calculator_used" || repo.lastMetadataKey != "from_currency" {
		t.Errorf("expected arguments to be forwarded, got type=%q name=%q key=%q", repo.lastType, repo.lastEventName, repo.lastMetadataKey)
	}
}

func TestGetMetadataBreakdown_DefaultsLimitAndRange(t *testing.T) {
	repo := &spyMetadataBreakdownRepo{}
	uc := usecase.NewGetMetadataBreakdown(repo)

	_, err := uc.Execute(context.Background(), usecase.GetMetadataBreakdownInput{ProjectID: "p1", EventName: "x", MetadataKey: "y"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastLimit != 10 {
		t.Errorf("expected default limit 10, got %d", repo.lastLimit)
	}
	span := repo.lastFilter.Until.Sub(repo.lastFilter.Since)
	if span < 6*24*time.Hour || span > 8*24*time.Hour {
		t.Errorf("expected roughly a 7 day span, got %v", span)
	}
}

func TestGetMetadataBreakdown_ClampsExcessiveLimit(t *testing.T) {
	repo := &spyMetadataBreakdownRepo{}
	uc := usecase.NewGetMetadataBreakdown(repo)

	_, err := uc.Execute(context.Background(), usecase.GetMetadataBreakdownInput{ProjectID: "p1", EventName: "x", MetadataKey: "y", Limit: 10000})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastLimit != 10 {
		t.Errorf("expected clamped limit 10, got %d", repo.lastLimit)
	}
}
