package httpapi_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/antoniojosev/traccia/internal/adapters/httpapi"
	"github.com/antoniojosev/traccia/internal/adapters/ratelimit"
	"github.com/antoniojosev/traccia/internal/usecase"
)

func newTestRouter(t *testing.T, rateLimitPerMinute int) http.Handler {
	t.Helper()
	return newTestRouterWithPing(t, rateLimitPerMinute, nil)
}

func newTestRouterWithPing(t *testing.T, rateLimitPerMinute int, ping func(context.Context) error) http.Handler {
	t.Helper()
	projects := newFakeProjectRepo()
	events := &fakeEventRepo{}
	visitors := &fakeVisitorRepo{}

	return httpapi.NewRouter(httpapi.Deps{
		AdminToken:      "admin-token",
		Auth:            usecase.NewAuthenticateProject(projects, fakeKeyHasher{}),
		CreateProject:   usecase.NewCreateProject(projects, fakeKeyHasher{}),
		TrackEvent:      usecase.NewTrackEvent(events, fakeUAParser{}, fakeGeoResolver{}),
		IdentifyVisitor: usecase.NewIdentifyVisitor(visitors),
		GetStats:        usecase.NewGetStats(events),
		RateLimiter:     ratelimit.New(rateLimitPerMinute),
		Ping:            ping,
	})
}

// newTestRouterWithProject builds a router with the given stats rate limit
// and returns an API key valid against it, so rate-limit tests can reach
// the authenticated /api/v1/stats path.
func newTestRouterWithProject(t *testing.T, rateLimitPerMinute int) (http.Handler, string) {
	t.Helper()
	projects := newFakeProjectRepo()
	events := &fakeEventRepo{}
	visitors := &fakeVisitorRepo{}
	create := usecase.NewCreateProject(projects, fakeKeyHasher{})

	_, apiKey, err := create.Execute(context.Background(), "Test Site", "example.com")
	if err != nil {
		t.Fatalf("seeding project: %v", err)
	}

	router := httpapi.NewRouter(httpapi.Deps{
		AdminToken:      "admin-token",
		Auth:            usecase.NewAuthenticateProject(projects, fakeKeyHasher{}),
		CreateProject:   create,
		TrackEvent:      usecase.NewTrackEvent(events, fakeUAParser{}, fakeGeoResolver{}),
		IdentifyVisitor: usecase.NewIdentifyVisitor(visitors),
		GetStats:        usecase.NewGetStats(events),
		RateLimiter:     ratelimit.New(rateLimitPerMinute),
	})
	return router, apiKey
}

func TestRouter_CORSPreflight(t *testing.T) {
	router := newTestRouter(t, 120)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/track", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("expected wildcard CORS origin, got %q", got)
	}
}

func TestRouter_RateLimitReturns429WhenExceeded(t *testing.T) {
	router := newTestRouter(t, 1) // burst of 1

	body := `{"project_id":"p1","visitor_id":"v1"}`
	first := httptest.NewRequest(http.MethodPost, "/api/v1/track", bytes.NewBufferString(body))
	first.RemoteAddr = "203.0.113.10:1234"
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, first)
	if rec1.Code != http.StatusAccepted {
		t.Fatalf("expected first request to succeed with 202, got %d", rec1.Code)
	}

	second := httptest.NewRequest(http.MethodPost, "/api/v1/track", bytes.NewBufferString(body))
	second.RemoteAddr = "203.0.113.10:1234"
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, second)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second request to be rate limited with 429, got %d", rec2.Code)
	}
}

func TestRouter_StatsIsRateLimitedPerIP(t *testing.T) {
	router, apiKey := newTestRouterWithProject(t, 1) // burst of 1

	newReq := func() *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.RemoteAddr = "203.0.113.20:1234"
		return req
	}

	first := httptest.NewRecorder()
	router.ServeHTTP(first, newReq())
	if first.Code != http.StatusOK {
		t.Fatalf("expected first request to succeed with 200, got %d: %s", first.Code, first.Body.String())
	}

	second := httptest.NewRecorder()
	router.ServeHTTP(second, newReq())
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second request to be rate limited with 429, got %d", second.Code)
	}
}

func TestRouter_HealthzOK(t *testing.T) {
	router := newTestRouter(t, 120)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRouter_HealthzOKWhenDatabaseReachable(t *testing.T) {
	router := newTestRouterWithPing(t, 120, func(context.Context) error { return nil })

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRouter_HealthzFailsWhenDatabaseUnreachable(t *testing.T) {
	router := newTestRouterWithPing(t, 120, func(context.Context) error { return errors.New("connection refused") })

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestRouter_ServesTrackingScript(t *testing.T) {
	router := newTestRouter(t, 120)

	req := httptest.NewRequest(http.MethodGet, "/t.js", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.Len() == 0 {
		t.Error("expected non-empty tracking script body")
	}
}
