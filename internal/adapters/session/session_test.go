package session_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/antoniojosev/traccia/internal/adapters/session"
)

func TestManager_RoundTrip(t *testing.T) {
	m := session.New("test-secret", "traccia_session", "/dashboard")
	rec := httptest.NewRecorder()
	m.SetCookie(rec, "project-123")

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}

	subject, err := m.SubjectFromRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if subject != "project-123" {
		t.Errorf("expected project-123, got %q", subject)
	}
}

func TestManager_RejectsMissingCookie(t *testing.T) {
	m := session.New("test-secret", "traccia_session", "/dashboard")
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)

	if _, err := m.SubjectFromRequest(req); err != session.ErrInvalid {
		t.Errorf("expected ErrInvalid, got %v", err)
	}
}

func TestManager_RejectsTamperedCookie(t *testing.T) {
	m := session.New("test-secret", "traccia_session", "/dashboard")
	rec := httptest.NewRecorder()
	m.SetCookie(rec, "project-123")

	cookies := rec.Result().Cookies()
	cookies[0].Value += "tampered"

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req.AddCookie(cookies[0])

	if _, err := m.SubjectFromRequest(req); err != session.ErrInvalid {
		t.Errorf("expected ErrInvalid for tampered cookie, got %v", err)
	}
}

func TestManager_RejectsTokenSignedWithDifferentSecret(t *testing.T) {
	a := session.New("secret-a", "traccia_session", "/dashboard")
	b := session.New("secret-b", "traccia_session", "/dashboard")

	rec := httptest.NewRecorder()
	a.SetCookie(rec, "project-123")

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}

	if _, err := b.SubjectFromRequest(req); err != session.ErrInvalid {
		t.Errorf("expected ErrInvalid when verifying with a different secret, got %v", err)
	}
}

func TestManager_DifferentCookieNamesDoNotCollide(t *testing.T) {
	dashboardSessions := session.New("shared-secret", "traccia_session", "/dashboard")
	adminSessions := session.New("shared-secret", "traccia_admin_session", "/admin")

	rec := httptest.NewRecorder()
	dashboardSessions.SetCookie(rec, "project-123")

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}

	if _, err := adminSessions.SubjectFromRequest(req); err != session.ErrInvalid {
		t.Errorf("expected the admin manager to not recognize a dashboard cookie, got %v", err)
	}
}
