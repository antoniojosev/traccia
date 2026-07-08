package dashboard_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/antoniojosev/traccia/internal/adapters/dashboard"
	"github.com/antoniojosev/traccia/internal/usecase"
)

func newTestHandler(t *testing.T) (http.Handler, string /* apiKey */) {
	t.Helper()
	projects := newFakeProjectRepo()
	events := &fakeEventRepo{}

	create := usecase.NewCreateProject(projects, fakeKeyHasher{})
	_, apiKey, err := create.Execute(context.Background(), "Test Site", "example.com")
	if err != nil {
		t.Fatalf("creating test project: %v", err)
	}

	handler := dashboard.NewHandler(dashboard.Deps{
		Auth:       usecase.NewAuthenticateProject(projects, fakeKeyHasher{}),
		GetStats:   usecase.NewGetStats(events),
		GetSamples: usecase.NewGetEventSamples(events),
		Sessions:   dashboard.NewSessionManager("test-secret"),
	})

	return handler, apiKey
}

func loginAndGetCookie(t *testing.T, handler http.Handler, apiKey string) *http.Cookie {
	t.Helper()
	form := url.Values{"api_key": {apiKey}}
	req := httptest.NewRequest(http.MethodPost, "/dashboard/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect after login, got %d: %s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected a session cookie to be set")
	}
	return cookies[0]
}

func TestDashboard_RedirectsToLoginWithoutSession(t *testing.T) {
	handler, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/dashboard/login" {
		t.Errorf("expected redirect to /dashboard/login, got %q", loc)
	}
}

func TestDashboard_LoginRejectsWrongKey(t *testing.T) {
	handler, _ := newTestHandler(t)

	form := url.Values{"api_key": {"wrong-key"}}
	req := httptest.NewRequest(http.MethodPost, "/dashboard/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "inválida") {
		t.Errorf("expected error message in body, got: %s", rec.Body.String())
	}
}

func TestDashboard_LoginSucceedsAndOverviewRenders(t *testing.T) {
	handler, apiKey := newTestHandler(t)
	cookie := loginAndGetCookie(t, handler, apiKey)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Eventos totales") {
		t.Errorf("expected overview page content, got: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "<html") {
		t.Errorf("expected a full HTML page for a non-htmx request")
	}
}

func TestDashboard_HTMXRequestGetsFragmentOnly(t *testing.T) {
	handler, apiKey := newTestHandler(t)
	cookie := loginAndGetCookie(t, handler, apiKey)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req.AddCookie(cookie)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "<html") {
		t.Errorf("expected a fragment (no <html>) for an htmx request, got: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Eventos totales") {
		t.Errorf("expected fragment to still contain the stats content")
	}
}

func TestDashboard_EventDrilldownRejectsUnknownType(t *testing.T) {
	handler, apiKey := newTestHandler(t)
	cookie := loginAndGetCookie(t, handler, apiKey)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/events/bogus/some_event", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestDashboard_EventDrilldownRendersSamples(t *testing.T) {
	handler, apiKey := newTestHandler(t)
	cookie := loginAndGetCookie(t, handler, apiKey)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/events/custom/calculator_used", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "calculator_used") {
		t.Errorf("expected event name in drilldown output, got: %s", rec.Body.String())
	}
}

func TestDashboard_LogoutClearsCookieAndRedirects(t *testing.T) {
	handler, apiKey := newTestHandler(t)
	cookie := loginAndGetCookie(t, handler, apiKey)

	req := httptest.NewRequest(http.MethodPost, "/dashboard/logout", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rec.Code)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) == 0 || cookies[0].MaxAge >= 0 {
		t.Errorf("expected logout to clear the session cookie (negative MaxAge), got %+v", cookies)
	}
}

func TestDashboard_ServesStaticAssets(t *testing.T) {
	handler, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/static/style.css", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.Len() == 0 {
		t.Error("expected non-empty CSS body")
	}
}
