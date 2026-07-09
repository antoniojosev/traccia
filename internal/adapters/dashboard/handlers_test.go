package dashboard_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/antoniojosev/traccia/internal/adapters/dashboard"
	"github.com/antoniojosev/traccia/internal/adapters/ratelimit"
	"github.com/antoniojosev/traccia/internal/adapters/session"
	"github.com/antoniojosev/traccia/internal/domain"
	"github.com/antoniojosev/traccia/internal/usecase"
)

func newTestHandler(t *testing.T) (http.Handler, string /* apiKey */) {
	return newTestHandlerWithPanels(t, nil)
}

func newTestHandlerWithPanels(t *testing.T, panels []dashboard.PanelView) (http.Handler, string /* apiKey */) {
	return newTestHandlerWithLoginLimit(t, panels, 1000)
}

func newTestHandlerWithLoginLimit(t *testing.T, panels []dashboard.PanelView, loginRateLimitPerMinute int) (http.Handler, string /* apiKey */) {
	t.Helper()
	projects := newFakeProjectRepo()
	events := &fakeEventRepo{}

	create := usecase.NewCreateProject(projects, fakeKeyHasher{})
	_, apiKey, err := create.Execute(context.Background(), "Test Site", "example.com")
	if err != nil {
		t.Fatalf("creating test project: %v", err)
	}

	handler := dashboard.NewHandler(dashboard.Deps{
		Auth:                 usecase.NewAuthenticateProject(projects, fakeKeyHasher{}),
		GetStats:             usecase.NewGetStats(events),
		GetSamples:           usecase.NewGetEventSamples(events),
		GetMetadataBreakdown: usecase.NewGetMetadataBreakdown(events),
		Sessions:             session.New("test-secret", "traccia_session", "/dashboard"),
		LoginLimiter:         ratelimit.New(loginRateLimitPerMinute),
		Panels:               panels,
	})

	return handler, apiKey
}

// newTestHandlerWithAdmin wires the project switcher's dependencies (a
// second, independent session.Manager standing in for the admin panel's
// sessions, plus ListProjects/GetProject) so tests can exercise it without
// pulling in the admin package itself.
func newTestHandlerWithAdmin(t *testing.T) (handler http.Handler, apiKey string, secondProjectID string, adminSessions *session.Manager) {
	t.Helper()
	projects := newFakeProjectRepo()
	events := &fakeEventRepo{}

	create := usecase.NewCreateProject(projects, fakeKeyHasher{})
	_, apiKey, err := create.Execute(context.Background(), "Test Site", "example.com")
	if err != nil {
		t.Fatalf("creating first test project: %v", err)
	}
	second, _, err := create.Execute(context.Background(), "Second Site", "second.example.com")
	if err != nil {
		t.Fatalf("creating second test project: %v", err)
	}

	adminSessions = session.New("admin-secret", "traccia_admin_session", "/admin")
	handler = dashboard.NewHandler(dashboard.Deps{
		Auth:                 usecase.NewAuthenticateProject(projects, fakeKeyHasher{}),
		GetStats:             usecase.NewGetStats(events),
		GetSamples:           usecase.NewGetEventSamples(events),
		GetMetadataBreakdown: usecase.NewGetMetadataBreakdown(events),
		Sessions:             session.New("test-secret", "traccia_session", "/dashboard"),
		LoginLimiter:         ratelimit.New(1000),
		AdminSessions:        adminSessions,
		ListProjects:         usecase.NewListProjects(projects),
		GetProject:           usecase.NewGetProject(projects),
	})

	return handler, apiKey, second.ID, adminSessions
}

func adminSessionCookie(adminSessions *session.Manager) *http.Cookie {
	rec := httptest.NewRecorder()
	adminSessions.SetCookie(rec, "some-admin-id")
	return rec.Result().Cookies()[0]
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

func TestDashboard_RendersPluginPanels(t *testing.T) {
	handler, apiKey := newTestHandlerWithPanels(t, []dashboard.PanelView{
		{Title: "Calculator usage", Kind: "line"},
	})
	cookie := loginAndGetCookie(t, handler, apiKey)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Calculator usage") {
		t.Errorf("expected plugin panel title in output, got: %s", rec.Body.String())
	}
}

func TestDashboard_PluginPanelWithGroupByRendersComputedRows(t *testing.T) {
	projects := newFakeProjectRepo()
	events := &fakeEventRepo{}
	events.metadataBreakdown = []domain.NameCount{{Name: "USD", Count: 42}}

	create := usecase.NewCreateProject(projects, fakeKeyHasher{})
	_, apiKey, err := create.Execute(context.Background(), "Test Site", "example.com")
	if err != nil {
		t.Fatalf("creating test project: %v", err)
	}

	handler := dashboard.NewHandler(dashboard.Deps{
		Auth:                 usecase.NewAuthenticateProject(projects, fakeKeyHasher{}),
		GetStats:             usecase.NewGetStats(events),
		GetSamples:           usecase.NewGetEventSamples(events),
		GetMetadataBreakdown: usecase.NewGetMetadataBreakdown(events),
		Sessions:             session.New("test-secret", "traccia_session", "/dashboard"),
		LoginLimiter:         ratelimit.New(1000),
		Panels: []dashboard.PanelView{
			{Title: "Calculator usage", Kind: "table", EventName: "calculator_used", GroupBy: "from_currency"},
		},
	})

	cookie := loginAndGetCookie(t, handler, apiKey)
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "USD") || !strings.Contains(rec.Body.String(), "42") {
		t.Errorf("expected computed breakdown row (USD, 42) in output, got: %s", rec.Body.String())
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

func TestDashboard_OverviewHidesSwitcherWithoutAdminSession(t *testing.T) {
	handler, apiKey, _, _ := newTestHandlerWithAdmin(t)
	cookie := loginAndGetCookie(t, handler, apiKey)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "Second Site") {
		t.Errorf("expected the switcher to stay hidden without an admin session, got: %s", rec.Body.String())
	}
}

func TestDashboard_OverviewShowsSwitcherWithAdminSession(t *testing.T) {
	handler, apiKey, _, adminSessions := newTestHandlerWithAdmin(t)
	cookie := loginAndGetCookie(t, handler, apiKey)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req.AddCookie(cookie)
	req.AddCookie(adminSessionCookie(adminSessions))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Second Site") {
		t.Errorf("expected the switcher to list every project with a valid admin session, got: %s", rec.Body.String())
	}
}

func TestDashboard_SwitchProjectRequiresAdminSession(t *testing.T) {
	handler, _, secondProjectID, _ := newTestHandlerWithAdmin(t)

	form := url.Values{"project_id": {secondProjectID}}
	req := httptest.NewRequest(http.MethodPost, "/dashboard/switch", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without an admin session, got %d", rec.Code)
	}
}

func TestDashboard_SwitchProjectMintsSessionForTargetProject(t *testing.T) {
	handler, _, secondProjectID, adminSessions := newTestHandlerWithAdmin(t)

	form := url.Values{"project_id": {secondProjectID}}
	req := httptest.NewRequest(http.MethodPost, "/dashboard/switch", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(adminSessionCookie(adminSessions))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/dashboard" {
		t.Fatalf("expected redirect to /dashboard, got %d %q: %s", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}

	var dashboardCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "traccia_session" {
			dashboardCookie = c
		}
	}
	if dashboardCookie == nil {
		t.Fatal("expected a dashboard session cookie to be set")
	}

	verifyReq := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	verifyReq.AddCookie(dashboardCookie)
	verifyRec := httptest.NewRecorder()
	handler.ServeHTTP(verifyRec, verifyReq)
	if verifyRec.Code != http.StatusOK {
		t.Fatalf("expected the new session to work against /dashboard, got %d", verifyRec.Code)
	}
}

func TestDashboard_SwitchProjectRejectsUnknownProject(t *testing.T) {
	handler, _, _, adminSessions := newTestHandlerWithAdmin(t)

	form := url.Values{"project_id": {"does-not-exist"}}
	req := httptest.NewRequest(http.MethodPost, "/dashboard/switch", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(adminSessionCookie(adminSessions))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestDashboard_LoginIsRateLimitedPerIP(t *testing.T) {
	handler, _ := newTestHandlerWithLoginLimit(t, nil, 1) // burst of 1

	form := url.Values{"api_key": {"wrong-key"}}
	newReq := func() *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/dashboard/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.RemoteAddr = "203.0.113.5:1234"
		return req
	}

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, newReq())
	if first.Code != http.StatusUnauthorized {
		t.Fatalf("expected first attempt to reach the handler (401 for wrong key), got %d", first.Code)
	}

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, newReq())
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second attempt to be rate limited with 429, got %d", second.Code)
	}
}
