package admin_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/antoniojosev/traccia/internal/adapters/admin"
	"github.com/antoniojosev/traccia/internal/adapters/session"
	"github.com/antoniojosev/traccia/internal/usecase"
)

const (
	testUsername = "antonio"
	testPassword = "correct-horse-battery"
)

func newTestHandler(t *testing.T) (http.Handler, *fakeProjectRepo, *fakeAdminUserRepo, *session.Manager) {
	t.Helper()
	projects := newFakeProjectRepo()
	adminUsers := newFakeAdminUserRepo()
	dashboardSessions := session.New("shared-secret", "traccia_session", "/dashboard")

	handler := admin.NewHandler(admin.Deps{
		Sessions:              session.New("shared-secret", "traccia_admin_session", "/admin"),
		RegisterAdminUser:     usecase.NewRegisterAdminUser(adminUsers, fakePasswordHasher{}),
		AuthenticateAdminUser: usecase.NewAuthenticateAdminUser(adminUsers, fakePasswordHasher{}),
		NeedsSetup:            usecase.NewNeedsAdminSetup(adminUsers),
		CreateProject:         usecase.NewCreateProject(projects, fakeKeyHasher{}),
		ListProjects:          usecase.NewListProjects(projects),
		GetProject:            usecase.NewGetProject(projects),
		DashboardSessions:     dashboardSessions,
	})

	return handler, projects, adminUsers, dashboardSessions
}

// seedAdminUser registers the account directly through the usecase, so
// tests that don't care about the setup flow itself can skip straight to
// a working login.
func seedAdminUser(t *testing.T, adminUsers *fakeAdminUserRepo) {
	t.Helper()
	register := usecase.NewRegisterAdminUser(adminUsers, fakePasswordHasher{})
	if _, err := register.Execute(context.Background(), testUsername, testPassword); err != nil {
		t.Fatalf("seeding admin user: %v", err)
	}
}

func adminLoginCookie(t *testing.T, handler http.Handler) *http.Cookie {
	t.Helper()
	form := url.Values{"username": {testUsername}, "password": {testPassword}}
	req := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 after login, got %d: %s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected a session cookie")
	}
	return cookies[0]
}

func TestAdmin_RedirectsToSetupWhenNoAccountExists(t *testing.T) {
	handler, _, _, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin/setup" {
		t.Fatalf("expected redirect to /admin/setup, got %d %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestAdmin_LoginPageRedirectsToSetupWhenNoAccountExists(t *testing.T) {
	handler, _, _, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/login", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin/setup" {
		t.Fatalf("expected redirect to /admin/setup, got %d %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestAdmin_SetupPageRedirectsToLoginOnceAnAccountExists(t *testing.T) {
	handler, _, adminUsers, _ := newTestHandler(t)
	seedAdminUser(t, adminUsers)

	req := httptest.NewRequest(http.MethodGet, "/admin/setup", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin/login" {
		t.Fatalf("expected redirect to /admin/login, got %d %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestAdmin_SetupCreatesAccountAndLogsIn(t *testing.T) {
	handler, _, _, _ := newTestHandler(t)

	form := url.Values{"username": {"antonio"}, "password": {"correct-horse-battery"}, "password_confirm": {"correct-horse-battery"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/setup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin" {
		t.Fatalf("expected redirect to /admin, got %d %q: %s", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
	if len(rec.Result().Cookies()) == 0 {
		t.Error("expected setup to log the new account in immediately")
	}
}

func TestAdmin_SetupRejectsMismatchedPasswords(t *testing.T) {
	handler, _, _, _ := newTestHandler(t)

	form := url.Values{"username": {"antonio"}, "password": {"correct-horse-battery"}, "password_confirm": {"something-else"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/setup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "no coinciden") {
		t.Errorf("expected mismatch error message, got: %s", rec.Body.String())
	}
}

func TestAdmin_SetupRejectsShortPassword(t *testing.T) {
	handler, _, _, _ := newTestHandler(t)

	form := url.Values{"username": {"antonio"}, "password": {"short"}, "password_confirm": {"short"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/setup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestAdmin_SetupSubmitRedirectsToLoginIfAccountAlreadyExists(t *testing.T) {
	handler, _, adminUsers, _ := newTestHandler(t)
	seedAdminUser(t, adminUsers)

	form := url.Values{"username": {"someone-else"}, "password": {"another-password"}, "password_confirm": {"another-password"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/setup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin/login" {
		t.Fatalf("expected redirect to /admin/login, got %d %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestAdmin_LoginRejectsWrongPassword(t *testing.T) {
	handler, _, adminUsers, _ := newTestHandler(t)
	seedAdminUser(t, adminUsers)

	form := url.Values{"username": {testUsername}, "password": {"wrong-password"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "incorrectos") {
		t.Errorf("expected error message, got: %s", rec.Body.String())
	}
}

func TestAdmin_ListsProjectsAfterLogin(t *testing.T) {
	handler, projects, adminUsers, _ := newTestHandler(t)
	seedAdminUser(t, adminUsers)
	create := usecase.NewCreateProject(projects, fakeKeyHasher{})
	if _, _, err := create.Execute(context.Background(), "My Site", "example.com"); err != nil {
		t.Fatalf("seeding project: %v", err)
	}

	cookie := adminLoginCookie(t, handler)
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "My Site") {
		t.Errorf("expected project name in listing, got: %s", rec.Body.String())
	}
}

func TestAdmin_ListShowsEmptyStateWithNoProjects(t *testing.T) {
	handler, _, adminUsers, _ := newTestHandler(t)
	seedAdminUser(t, adminUsers)
	cookie := adminLoginCookie(t, handler)

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Todavía no hay proyectos") {
		t.Errorf("expected empty state message, got: %s", rec.Body.String())
	}
}

func TestAdmin_CreatesProjectAndRevealsKeyOnce(t *testing.T) {
	handler, projects, adminUsers, _ := newTestHandler(t)
	seedAdminUser(t, adminUsers)
	cookie := adminLoginCookie(t, handler)

	form := url.Values{"name": {"New Site"}, "domain": {"new.example.com"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/projects/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "plain-key") {
		t.Errorf("expected the plaintext API key to be shown, got: %s", rec.Body.String())
	}
	if list, _ := projects.List(context.Background()); len(list) != 1 {
		t.Errorf("expected 1 project persisted, got %d", len(list))
	}
}

func TestAdmin_RejectsProjectCreationWithoutName(t *testing.T) {
	handler, _, adminUsers, _ := newTestHandler(t)
	seedAdminUser(t, adminUsers)
	cookie := adminLoginCookie(t, handler)

	form := url.Values{"domain": {"example.com"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/projects/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestAdmin_ViewProjectMintsDashboardSessionAndRedirects(t *testing.T) {
	handler, projects, adminUsers, dashboardSessions := newTestHandler(t)
	seedAdminUser(t, adminUsers)
	create := usecase.NewCreateProject(projects, fakeKeyHasher{})
	created, _, err := create.Execute(context.Background(), "My Site", "example.com")
	if err != nil {
		t.Fatalf("seeding project: %v", err)
	}

	cookie := adminLoginCookie(t, handler)
	req := httptest.NewRequest(http.MethodPost, "/admin/projects/"+created.ID+"/view", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/dashboard" {
		t.Fatalf("expected redirect to /dashboard, got %d %q", rec.Code, rec.Header().Get("Location"))
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
	subject, err := dashboardSessions.SubjectFromRequest(verifyReq)
	if err != nil {
		t.Fatalf("dashboard session should verify: %v", err)
	}
	if subject != created.ID {
		t.Errorf("expected dashboard session for project %q, got %q", created.ID, subject)
	}
}

func TestAdmin_ViewProjectRejectsUnknownID(t *testing.T) {
	handler, _, adminUsers, _ := newTestHandler(t)
	seedAdminUser(t, adminUsers)
	cookie := adminLoginCookie(t, handler)

	req := httptest.NewRequest(http.MethodPost, "/admin/projects/does-not-exist/view", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestAdmin_LogoutClearsSessionCookie(t *testing.T) {
	handler, _, adminUsers, _ := newTestHandler(t)
	seedAdminUser(t, adminUsers)
	cookie := adminLoginCookie(t, handler)

	req := httptest.NewRequest(http.MethodPost, "/admin/logout", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rec.Code)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 || cookies[0].MaxAge >= 0 {
		t.Errorf("expected logout to clear the session cookie, got %+v", cookies)
	}
}
