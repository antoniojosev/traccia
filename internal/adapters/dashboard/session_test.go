package dashboard_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/antoniojosev/traccia/internal/adapters/dashboard"
)

func TestSessionManager_RoundTrip(t *testing.T) {
	sm := dashboard.NewSessionManager("test-secret")
	rec := httptest.NewRecorder()
	sm.SetCookie(rec, "project-123")

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}

	projectID, err := sm.ProjectIDFromRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if projectID != "project-123" {
		t.Errorf("expected project-123, got %q", projectID)
	}
}

func TestSessionManager_RejectsMissingCookie(t *testing.T) {
	sm := dashboard.NewSessionManager("test-secret")
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)

	if _, err := sm.ProjectIDFromRequest(req); err != dashboard.ErrInvalidSession {
		t.Errorf("expected ErrInvalidSession, got %v", err)
	}
}

func TestSessionManager_RejectsTamperedCookie(t *testing.T) {
	sm := dashboard.NewSessionManager("test-secret")
	rec := httptest.NewRecorder()
	sm.SetCookie(rec, "project-123")

	cookies := rec.Result().Cookies()
	cookies[0].Value += "tampered"

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req.AddCookie(cookies[0])

	if _, err := sm.ProjectIDFromRequest(req); err != dashboard.ErrInvalidSession {
		t.Errorf("expected ErrInvalidSession for tampered cookie, got %v", err)
	}
}

func TestSessionManager_RejectsTokenSignedWithDifferentSecret(t *testing.T) {
	smA := dashboard.NewSessionManager("secret-a")
	smB := dashboard.NewSessionManager("secret-b")

	rec := httptest.NewRecorder()
	smA.SetCookie(rec, "project-123")

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}

	if _, err := smB.ProjectIDFromRequest(req); err != dashboard.ErrInvalidSession {
		t.Errorf("expected ErrInvalidSession when verifying with a different secret, got %v", err)
	}
}
