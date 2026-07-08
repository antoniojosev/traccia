package httpapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/antoniojosev/traccia/internal/adapters/httpapi"
	"github.com/antoniojosev/traccia/internal/usecase"
)

// setupProject creates a project through the real usecases (backed by
// fakes) and returns its plaintext API key, so handler tests exercise the
// exact same auth path production traffic does.
func setupProject(t *testing.T) (projectID, apiKey string, projects *fakeProjectRepo) {
	t.Helper()
	projects = newFakeProjectRepo()
	create := usecase.NewCreateProject(projects, fakeKeyHasher{})
	project, plainKey, err := create.Execute(context.Background(), "Test Site", "example.com")
	if err != nil {
		t.Fatalf("unexpected error creating project: %v", err)
	}
	return project.ID, plainKey, projects
}

func TestHandleStats_RejectsMissingKey(t *testing.T) {
	_, _, projects := setupProject(t)
	events := &fakeEventRepo{}
	auth := usecase.NewAuthenticateProject(projects, fakeKeyHasher{})
	handler := httpapi.HandleStats(auth, usecase.NewGetStats(events))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestHandleStats_RejectsWrongKey(t *testing.T) {
	_, _, projects := setupProject(t)
	events := &fakeEventRepo{}
	auth := usecase.NewAuthenticateProject(projects, fakeKeyHasher{})
	handler := httpapi.HandleStats(auth, usecase.NewGetStats(events))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestHandleStats_AcceptsCorrectKey(t *testing.T) {
	_, apiKey, projects := setupProject(t)
	events := &fakeEventRepo{}
	auth := usecase.NewAuthenticateProject(projects, fakeKeyHasher{})
	handler := httpapi.HandleStats(auth, usecase.NewGetStats(events))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleStats_AcceptsApiKeyHeaderAsFallback(t *testing.T) {
	_, apiKey, projects := setupProject(t)
	events := &fakeEventRepo{}
	auth := usecase.NewAuthenticateProject(projects, fakeKeyHasher{})
	handler := httpapi.HandleStats(auth, usecase.NewGetStats(events))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	req.Header.Set("Api-Key", apiKey)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
