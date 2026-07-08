package httpapi_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/antoniojosev/traccia/internal/adapters/httpapi"
	"github.com/antoniojosev/traccia/internal/usecase"
)

func TestHandleCreateProject_RejectsWithoutAdminToken(t *testing.T) {
	projects := newFakeProjectRepo()
	create := usecase.NewCreateProject(projects, fakeKeyHasher{})
	handler := httpapi.HandleCreateProject("secret-admin-token", create)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBufferString(`{"name":"x"}`))
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestHandleCreateProject_RejectsWrongAdminToken(t *testing.T) {
	projects := newFakeProjectRepo()
	create := usecase.NewCreateProject(projects, fakeKeyHasher{})
	handler := httpapi.HandleCreateProject("secret-admin-token", create)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBufferString(`{"name":"x"}`))
	req.Header.Set("Authorization", "Bearer wrong-token")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestHandleCreateProject_CreatesWithCorrectAdminToken(t *testing.T) {
	projects := newFakeProjectRepo()
	create := usecase.NewCreateProject(projects, fakeKeyHasher{})
	handler := httpapi.HandleCreateProject("secret-admin-token", create)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBufferString(`{"name":"My Site","domain":"example.com"}`))
	req.Header.Set("Authorization", "Bearer secret-admin-token")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(projects.byID) != 1 {
		t.Fatalf("expected 1 project created, got %d", len(projects.byID))
	}
}

func TestHandleCreateProject_RejectsMissingName(t *testing.T) {
	projects := newFakeProjectRepo()
	create := usecase.NewCreateProject(projects, fakeKeyHasher{})
	handler := httpapi.HandleCreateProject("secret-admin-token", create)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBufferString(`{}`))
	req.Header.Set("Authorization", "Bearer secret-admin-token")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
