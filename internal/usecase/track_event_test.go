package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/antoniojosev/traccia/internal/domain"
	"github.com/antoniojosev/traccia/internal/usecase"
)

func TestTrackEvent_RequiresProjectAndVisitor(t *testing.T) {
	uc := usecase.NewTrackEvent(&fakeEventRepo{}, fakeUAParser{}, fakeGeoResolver{})

	err := uc.Execute(context.Background(), usecase.TrackEventInput{})

	if !errors.Is(err, usecase.ErrInvalidEvent) {
		t.Fatalf("expected ErrInvalidEvent, got %v", err)
	}
}

func TestTrackEvent_CustomTypeRequiresName(t *testing.T) {
	uc := usecase.NewTrackEvent(&fakeEventRepo{}, fakeUAParser{}, fakeGeoResolver{})

	err := uc.Execute(context.Background(), usecase.TrackEventInput{
		ProjectID: "p1",
		VisitorID: "v1",
		Type:      domain.EventTypeCustom,
	})

	if !errors.Is(err, usecase.ErrInvalidEvent) {
		t.Fatalf("expected ErrInvalidEvent for custom event without name, got %v", err)
	}
}

func TestTrackEvent_DefaultsToPageviewAndAnonymizesIP(t *testing.T) {
	repo := &fakeEventRepo{}
	uc := usecase.NewTrackEvent(repo, fakeUAParser{}, fakeGeoResolver{})

	err := uc.Execute(context.Background(), usecase.TrackEventInput{
		ProjectID: "p1",
		VisitorID: "v1",
		Path:      "/calculator",
		IP:        "203.0.113.42",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.saved) != 1 {
		t.Fatalf("expected 1 saved event, got %d", len(repo.saved))
	}

	got := repo.saved[0]
	if got.Type != domain.EventTypePageview {
		t.Errorf("expected default type pageview, got %q", got.Type)
	}
	if got.IPAnonymized != "203.0.113.0" {
		t.Errorf("expected last octet zeroed, got %q", got.IPAnonymized)
	}
	if got.Geo.Country != "VE" {
		t.Errorf("expected geo resolver result to be used, got %q", got.Geo.Country)
	}
}
