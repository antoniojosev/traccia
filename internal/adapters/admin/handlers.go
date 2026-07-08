// Package admin is the control-plane panel: create/list projects and jump
// into a project's dashboard, gated by ADMIN_TOKEN — a different trust
// level than the dashboard package's per-project API key sessions. It's
// the human-friendly alternative to POST /api/v1/projects via curl, not a
// replacement for it (scripts/automation still use the API).
package admin

import (
	"crypto/subtle"
	"html/template"
	"net/http"

	"github.com/antoniojosev/traccia/internal/adapters/session"
	"github.com/antoniojosev/traccia/internal/usecase"
)

type Deps struct {
	AdminToken    string
	Sessions      *session.Manager
	CreateProject *usecase.CreateProject
	ListProjects  *usecase.ListProjects
	GetProject    *usecase.GetProject
	// DashboardSessions mints a dashboard login session directly — the
	// admin already proved elevated trust via AdminToken, so "view this
	// project's dashboard" doesn't require re-entering that project's API
	// key (which the admin panel never has in plaintext anyway).
	DashboardSessions *session.Manager
}

func NewHandler(deps Deps) http.Handler {
	tmpl := parseTemplates()
	mux := http.NewServeMux()

	mux.HandleFunc("GET /admin/login", handleLoginPage(tmpl))
	mux.HandleFunc("POST /admin/login", handleLoginSubmit(deps, tmpl))
	mux.HandleFunc("POST /admin/logout", handleLogout(deps))
	mux.HandleFunc("GET /admin", requireAdminSession(deps, handleProjectsList(deps, tmpl)))
	mux.HandleFunc("GET /admin/projects/new", requireAdminSession(deps, handleNewProjectPage(tmpl)))
	mux.HandleFunc("POST /admin/projects/new", requireAdminSession(deps, handleNewProjectSubmit(deps, tmpl)))
	mux.HandleFunc("POST /admin/projects/{id}/view", requireAdminSession(deps, handleViewProject(deps)))
	mux.Handle("GET /admin/static/", staticHandler())

	return mux
}

const adminSubject = "admin"

func requireAdminSession(deps Deps, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := deps.Sessions.SubjectFromRequest(r); err != nil {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

type loginView struct {
	Error string
}

func handleLoginPage(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tmpl.ExecuteTemplate(w, "admin-login-page", loginView{})
	}
}

func handleLoginSubmit(deps Deps, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}

		token := r.FormValue("admin_token")
		if deps.AdminToken == "" || subtle.ConstantTimeCompare([]byte(token), []byte(deps.AdminToken)) != 1 {
			w.WriteHeader(http.StatusUnauthorized)
			tmpl.ExecuteTemplate(w, "admin-login-page", loginView{Error: "ADMIN_TOKEN inválido."})
			return
		}

		deps.Sessions.SetCookie(w, adminSubject)
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
	}
}

func handleLogout(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deps.Sessions.ClearCookie(w)
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
	}
}

type projectView struct {
	ID        string
	Name      string
	Domain    string
	CreatedAt string
}

type projectsListView struct {
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

		tmpl.ExecuteTemplate(w, "admin-projects-page", projectsListView{Projects: views})
	}
}

type newProjectView struct {
	Error string
}

func handleNewProjectPage(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tmpl.ExecuteTemplate(w, "admin-new-project-page", newProjectView{})
	}
}

type projectCreatedView struct {
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
			tmpl.ExecuteTemplate(w, "admin-new-project-page", newProjectView{Error: "El nombre es obligatorio."})
			return
		}

		project, apiKey, err := deps.CreateProject.Execute(r.Context(), name, r.FormValue("domain"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		tmpl.ExecuteTemplate(w, "admin-project-created-page", projectCreatedView{
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
