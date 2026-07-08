package httpapi_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/antoniojosev/traccia/internal/adapters/httpapi"
	"github.com/antoniojosev/traccia/internal/usecase"
)

func TestHandleTrack_HappyPath(t *testing.T) {
	events := &fakeEventRepo{}
	track := usecase.NewTrackEvent(events, fakeUAParser{}, fakeGeoResolver{})
	handler := httpapi.HandleTrack(track)

	body := `{"project_id":"p1","visitor_id":"v1","path":"/calculator"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/track", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(events.saved) != 1 {
		t.Fatalf("expected 1 saved event, got %d", len(events.saved))
	}
}

func TestHandleTrack_RejectsInvalidBody(t *testing.T) {
	events := &fakeEventRepo{}
	track := usecase.NewTrackEvent(events, fakeUAParser{}, fakeGeoResolver{})
	handler := httpapi.HandleTrack(track)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/track", bytes.NewBufferString("not json"))
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleTrack_RejectsMissingProjectOrVisitor(t *testing.T) {
	events := &fakeEventRepo{}
	track := usecase.NewTrackEvent(events, fakeUAParser{}, fakeGeoResolver{})
	handler := httpapi.HandleTrack(track)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/track", bytes.NewBufferString(`{}`))
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if len(events.saved) != 0 {
		t.Fatalf("expected no event saved, got %d", len(events.saved))
	}
}

func TestHandleIdentify_HappyPath(t *testing.T) {
	visitors := &fakeVisitorRepo{}
	identify := usecase.NewIdentifyVisitor(visitors)
	handler := httpapi.HandleIdentify(identify)

	body := `{"project_id":"p1","visitor_id":"v1","name":"Antonio (yo mismo)"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/identify", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(visitors.upserted) != 1 || visitors.upserted[0].Name != "Antonio (yo mismo)" {
		t.Fatalf("expected 1 upsert with name, got %+v", visitors.upserted)
	}
}
