// Package admin is the control-plane panel: create/list projects and jump
// into a project's dashboard. Gated by its own human accounts (username +
// password, one-time setup, see NeedsAdminSetup) — a different, more
// privileged trust level than the dashboard package's per-project API key
// sessions. It's the human-friendly alternative to POST /api/v1/projects
// via curl, not a replacement for it (scripts/automation still use the
// API with ADMIN_TOKEN, which this panel no longer touches at all).
package admin

import (
	"context"
	"errors"
	"html/template"
	"net/http"

	"github.com/antoniojosev/traccia/internal/adapters/ratelimit"
	"github.com/antoniojosev/traccia/internal/adapters/session"
	"github.com/antoniojosev/traccia/internal/usecase"
)

type Deps struct {
	Sessions              *session.Manager
	RegisterAdminUser     *usecase.RegisterAdminUser
	AuthenticateAdminUser *usecase.AuthenticateAdminUser
	NeedsSetup            *usecase.NeedsAdminSetup
	CreateProject         *usecase.CreateProject
	ListProjects          *usecase.ListProjects
	GetProject            *usecase.GetProject
	DeleteProject         *usecase.DeleteProject
	UpdateProject         *usecase.UpdateProject
	RotateAPIKey          *usecase.RotateAPIKey
	AddAdminUser          *usecase.AddAdminUser
	ListAdminUsers        *usecase.ListAdminUsers
	GetAdminUser          *usecase.GetAdminUser
	DeleteAdminUser       *usecase.DeleteAdminUser
	// DashboardSessions mints a dashboard login session directly — an
	// admin account already carries more trust than any single project's
	// API key (which this panel never handles in plaintext anyway), so
	// "view this project's dashboard" doesn't require re-entering it.
	DashboardSessions *session.Manager
	// LoginLimiter guards /admin/login (brute-forcing a password) and
	// /admin/setup (spamming bcrypt hashing / account creation attempts).
	LoginLimiter *ratelimit.Limiter
}

func NewHandler(deps Deps) http.Handler {
	tmpl := parseTemplates()
	mux := http.NewServeMux()

	mux.HandleFunc("GET /admin/setup", handleSetupPage(deps, tmpl))
	mux.HandleFunc("POST /admin/setup", withLoginRateLimit(deps.LoginLimiter, handleSetupSubmit(deps, tmpl)))
	mux.HandleFunc("GET /admin/login", handleLoginPage(deps, tmpl))
	mux.HandleFunc("POST /admin/login", withLoginRateLimit(deps.LoginLimiter, handleLoginSubmit(deps, tmpl)))
	mux.HandleFunc("POST /admin/logout", handleLogout(deps))
	mux.HandleFunc("GET /admin", requireAdminSession(deps, handleProjectsList(deps, tmpl)))
	mux.HandleFunc("GET /admin/projects/new", requireAdminSession(deps, handleNewProjectPage(tmpl)))
	mux.HandleFunc("POST /admin/projects/new", requireAdminSession(deps, handleNewProjectSubmit(deps, tmpl)))
	mux.HandleFunc("POST /admin/projects/{id}/view", requireAdminSession(deps, handleViewProject(deps)))
	mux.HandleFunc("GET /admin/projects/{id}/delete", requireAdminSession(deps, handleDeleteConfirm(deps, tmpl)))
	mux.HandleFunc("POST /admin/projects/{id}/delete", requireAdminSession(deps, handleDeleteSubmit(deps)))
	mux.HandleFunc("GET /admin/projects/{id}/edit", requireAdminSession(deps, handleEditProjectPage(deps, tmpl)))
	mux.HandleFunc("POST /admin/projects/{id}/edit", requireAdminSession(deps, handleEditProjectSubmit(deps, tmpl)))
	mux.HandleFunc("GET /admin/projects/{id}/rotate-key", requireAdminSession(deps, handleRotateKeyConfirm(deps, tmpl)))
	mux.HandleFunc("POST /admin/projects/{id}/rotate-key", requireAdminSession(deps, handleRotateKeySubmit(deps, tmpl)))
	mux.HandleFunc("GET /admin/users", requireAdminSession(deps, handleUsersList(deps, tmpl)))
	mux.HandleFunc("POST /admin/users/new", requireAdminSession(deps, handleAddUserSubmit(deps, tmpl)))
	mux.HandleFunc("GET /admin/users/{id}/delete", requireAdminSession(deps, handleDeleteUserConfirm(deps, tmpl)))
	mux.HandleFunc("POST /admin/users/{id}/delete", requireAdminSession(deps, handleDeleteUserSubmit(deps)))

	return mux
}

func withLoginRateLimit(rl *ratelimit.Limiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !rl.Allow(ratelimit.ClientIP(r)) {
			http.Error(w, "too many attempts, try again later", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

// requireAdminSession redirects to /admin/setup before /admin/login when
// no account exists yet — a fresh install should never show a login form
// for an account that can't possibly exist.
func requireAdminSession(deps Deps, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		needsSetup, err := deps.NeedsSetup.Execute(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if needsSetup {
			http.Redirect(w, r, "/admin/setup", http.StatusSeeOther)
			return
		}
		subject, err := deps.Sessions.SubjectFromRequest(r)
		if err != nil {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		// The session cookie is a stateless HMAC token valid for days, so a
		// deleted account would otherwise keep working until it expires —
		// re-check against the database on every request to revoke access
		// immediately when an admin is removed.
		if _, err := deps.GetAdminUser.Execute(r.Context(), subject); err != nil {
			deps.Sessions.ClearCookie(w)
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

type setupView struct {
	Error    string
	Username string
}

func handleSetupPage(deps Deps, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		needsSetup, err := deps.NeedsSetup.Execute(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !needsSetup {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		tmpl.ExecuteTemplate(w, "admin-setup-page", setupView{})
	}
}

func handleSetupSubmit(deps Deps, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		username := r.FormValue("username")
		password := r.FormValue("password")

		if password != r.FormValue("password_confirm") {
			w.WriteHeader(http.StatusBadRequest)
			tmpl.ExecuteTemplate(w, "admin-setup-page", setupView{
				Error: "Las contraseñas no coinciden.", Username: username,
			})
			return
		}

		user, err := deps.RegisterAdminUser.Execute(r.Context(), username, password)
		if err != nil {
			if errors.Is(err, usecase.ErrAdminSetupClosed) {
				http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
				return
			}
			w.WriteHeader(http.StatusBadRequest)
			tmpl.ExecuteTemplate(w, "admin-setup-page", setupView{
				Error:    "El usuario debe tener al menos 3 caracteres y la contraseña al menos 8.",
				Username: username,
			})
			return
		}

		deps.Sessions.SetCookie(w, user.ID)
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
	}
}

type loginView struct {
	Error    string
	Username string
}

func handleLoginPage(deps Deps, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		needsSetup, err := deps.NeedsSetup.Execute(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if needsSetup {
			http.Redirect(w, r, "/admin/setup", http.StatusSeeOther)
			return
		}
		tmpl.ExecuteTemplate(w, "admin-login-page", loginView{})
	}
}

func handleLoginSubmit(deps Deps, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}

		username := r.FormValue("username")
		user, err := deps.AuthenticateAdminUser.Execute(r.Context(), username, r.FormValue("password"))
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			tmpl.ExecuteTemplate(w, "admin-login-page", loginView{
				Error: "Usuario o contraseña incorrectos.", Username: username,
			})
			return
		}

		deps.Sessions.SetCookie(w, user.ID)
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
	}
}

func handleLogout(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deps.Sessions.ClearCookie(w)
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
	}
}

// nav carries which top-level section is active, so the shared "nav"
// template can mark the right link with aria-current — every view struct
// that renders {{template "nav" .}} embeds this.
type nav struct {
	Active string
}

type projectView struct {
	ID        string
	Name      string
	Domain    string
	CreatedAt string
}

type projectsListView struct {
	nav
	Projects []projectView
}

func handleProjectsList(deps Deps, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projects, err := deps.ListProjects.Execute(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		views := make([]projectView, 0, len(projects))
		for _, p := range projects {
			views = append(views, projectView{
				ID:        p.ID,
				Name:      p.Name,
				Domain:    p.Domain,
				CreatedAt: p.CreatedAt.Format("2006-01-02 15:04"),
			})
		}

		tmpl.ExecuteTemplate(w, "admin-projects-page", projectsListView{nav: nav{Active: "proyectos"}, Projects: views})
	}
}

type newProjectView struct {
	nav
	Error string
}

func handleNewProjectPage(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tmpl.ExecuteTemplate(w, "admin-new-project-page", newProjectView{nav: nav{Active: "proyectos"}})
	}
}

type projectCreatedView struct {
	nav
	ProjectID string
	APIKey    string
}

func handleNewProjectSubmit(deps Deps, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}

		name := r.FormValue("name")
		if name == "" {
			w.WriteHeader(http.StatusBadRequest)
			tmpl.ExecuteTemplate(w, "admin-new-project-page", newProjectView{nav: nav{Active: "proyectos"}, Error: "El nombre es obligatorio."})
			return
		}

		project, apiKey, err := deps.CreateProject.Execute(r.Context(), name, r.FormValue("domain"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		tmpl.ExecuteTemplate(w, "admin-project-created-page", projectCreatedView{
			nav:       nav{Active: "proyectos"},
			ProjectID: project.ID,
			APIKey:    apiKey,
		})
	}
}

func handleViewProject(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if _, err := deps.GetProject.Execute(r.Context(), id); err != nil {
			http.Error(w, "project not found", http.StatusNotFound)
			return
		}

		deps.DashboardSessions.SetCookie(w, id)
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	}
}

type deleteConfirmView struct {
	nav
	ID   string
	Name string
}

func handleDeleteConfirm(deps Deps, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		project, err := deps.GetProject.Execute(r.Context(), id)
		if err != nil {
			http.Error(w, "project not found", http.StatusNotFound)
			return
		}

		tmpl.ExecuteTemplate(w, "admin-delete-confirm-page", deleteConfirmView{nav: nav{Active: "proyectos"}, ID: project.ID, Name: project.Name})
	}
}

func handleDeleteSubmit(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if _, err := deps.GetProject.Execute(r.Context(), id); err != nil {
			http.Error(w, "project not found", http.StatusNotFound)
			return
		}
		if err := deps.DeleteProject.Execute(r.Context(), id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
	}
}

type editProjectView struct {
	nav
	ID     string
	Name   string
	Domain string
	Error  string
}

func handleEditProjectPage(deps Deps, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		project, err := deps.GetProject.Execute(r.Context(), id)
		if err != nil {
			http.Error(w, "project not found", http.StatusNotFound)
			return
		}
		tmpl.ExecuteTemplate(w, "admin-edit-project-page", editProjectView{
			nav: nav{Active: "proyectos"}, ID: project.ID, Name: project.Name, Domain: project.Domain,
		})
	}
}

func handleEditProjectSubmit(deps Deps, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if _, err := deps.GetProject.Execute(r.Context(), id); err != nil {
			http.Error(w, "project not found", http.StatusNotFound)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}

		name := r.FormValue("name")
		domainName := r.FormValue("domain")
		if _, err := deps.UpdateProject.Execute(r.Context(), id, name, domainName); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			tmpl.ExecuteTemplate(w, "admin-edit-project-page", editProjectView{
				nav: nav{Active: "proyectos"}, ID: id, Name: name, Domain: domainName,
				Error: "El nombre es obligatorio.",
			})
			return
		}

		http.Redirect(w, r, "/admin", http.StatusSeeOther)
	}
}

type rotateKeyConfirmView struct {
	nav
	ID   string
	Name string
}

func handleRotateKeyConfirm(deps Deps, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		project, err := deps.GetProject.Execute(r.Context(), id)
		if err != nil {
			http.Error(w, "project not found", http.StatusNotFound)
			return
		}
		tmpl.ExecuteTemplate(w, "admin-rotate-key-confirm-page", rotateKeyConfirmView{
			nav: nav{Active: "proyectos"}, ID: project.ID, Name: project.Name,
		})
	}
}

type keyRotatedView struct {
	nav
	ProjectID string
	APIKey    string
}

func handleRotateKeySubmit(deps Deps, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if _, err := deps.GetProject.Execute(r.Context(), id); err != nil {
			http.Error(w, "project not found", http.StatusNotFound)
			return
		}

		newKey, err := deps.RotateAPIKey.Execute(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		tmpl.ExecuteTemplate(w, "admin-key-rotated-page", keyRotatedView{
			nav: nav{Active: "proyectos"}, ProjectID: id, APIKey: newKey,
		})
	}
}

type adminUserView struct {
	ID        string
	Username  string
	CreatedAt string
}

type usersListView struct {
	nav
	Users    []adminUserView
	Error    string
	Username string
}

func handleUsersList(deps Deps, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		renderUsersList(r.Context(), w, tmpl, deps, usersListView{nav: nav{Active: "usuarios"}}, http.StatusOK)
	}
}

func renderUsersList(ctx context.Context, w http.ResponseWriter, tmpl *template.Template, deps Deps, view usersListView, status int) {
	users, err := deps.ListAdminUsers.Execute(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, u := range users {
		view.Users = append(view.Users, adminUserView{ID: u.ID, Username: u.Username, CreatedAt: u.CreatedAt.Format("2006-01-02 15:04")})
	}
	w.WriteHeader(status)
	tmpl.ExecuteTemplate(w, "admin-users-page", view)
}

func handleAddUserSubmit(deps Deps, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		username := r.FormValue("username")

		if _, err := deps.AddAdminUser.Execute(r.Context(), username, r.FormValue("password")); err != nil {
			message := "El usuario debe tener al menos 3 caracteres y la contraseña al menos 8."
			if errors.Is(err, usecase.ErrAdminUsernameTaken) {
				message = "Ese usuario ya existe."
			}
			renderUsersList(r.Context(), w, tmpl, deps, usersListView{
				nav: nav{Active: "usuarios"}, Error: message, Username: username,
			}, http.StatusBadRequest)
			return
		}

		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
	}
}

type deleteUserConfirmView struct {
	nav
	ID       string
	Username string
	Error    string
}

func handleDeleteUserConfirm(deps Deps, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		user, err := deps.GetAdminUser.Execute(r.Context(), id)
		if err != nil {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		tmpl.ExecuteTemplate(w, "admin-delete-user-confirm-page", deleteUserConfirmView{
			nav: nav{Active: "usuarios"}, ID: user.ID, Username: user.Username,
		})
	}
}

func handleDeleteUserSubmit(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if _, err := deps.GetAdminUser.Execute(r.Context(), id); err != nil {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		if err := deps.DeleteAdminUser.Execute(r.Context(), id); err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, usecase.ErrCannotDeleteLastAdmin) {
				status = http.StatusBadRequest
			}
			http.Error(w, err.Error(), status)
			return
		}
		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
	}
}
