package webui_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/antoniojosev/traccia/internal/adapters/webui"
)

func TestHandler_ServesTheme(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/assets/theme.css", nil)
	rec := httptest.NewRecorder()
	webui.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.Len() == 0 {
		t.Error("expected non-empty CSS body")
	}
}

func TestHandler_404sUnknownFile(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/assets/does-not-exist.css", nil)
	rec := httptest.NewRecorder()
	webui.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
