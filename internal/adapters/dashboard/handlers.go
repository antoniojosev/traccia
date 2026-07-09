package dashboard

import (
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"time"

	"github.com/antoniojosev/traccia/internal/adapters/ratelimit"
	"github.com/antoniojosev/traccia/internal/adapters/session"
	"github.com/antoniojosev/traccia/internal/domain"
	"github.com/antoniojosev/traccia/internal/usecase"
)

type Deps struct {
	Auth                 *usecase.AuthenticateProject
	GetStats             *usecase.GetStats
	GetSamples           *usecase.GetEventSamples
	GetMetadataBreakdown *usecase.GetMetadataBreakdown
	Sessions             *session.Manager
	// LoginLimiter is per-IP, much stricter than the ingest rate limit — a
	// login attempt should never be as frequent as a pageview, and this is
	// the only thing standing between an API key and brute-forcing it.
	LoginLimiter *ratelimit.Limiter
	// Panels are plugin-declared specs (see plugins.Manager.Panels),
	// rendered here in the core so a plugin never ships its own frontend
	// JS — the reason this dashboard is server-rendered HTMX instead of a
	// SPA. Rows are computed fresh per request in handleOverview, not
	// stored here — this slice is shared across every request.
	Panels []PanelView
	// AdminSessions, ListProjects and GetProject back the project switcher
	// shown in the topbar — deliberately gated on a valid *admin* session,
	// not the dashboard's own per-project one. A single project's API key
	// must never let its holder jump into another project's dashboard;
	// only someone who already holds the more privileged admin session
	// can. handleSwitchProject re-checks this itself server-side, so
	// leaving these nil (or a plain per-project dashboard deployment with
	// no admin panel wired up) just hides the switcher rather than weaken
	// anything.
	AdminSessions *session.Manager
	ListProjects  *usecase.ListProjects
	GetProject    *usecase.GetProject
}

// PanelView is a plugin-declared panel spec. EventName/GroupBy come
// straight from the plugin's registerPanel() and describe what to
// compute; Rows is filled in per-request by handleOverview, never at
// startup (Panels is shared across every request, so it must stay
// read-only template data — no field one request fills in can leak into
// another's response).
type PanelView struct {
	Title     string
	Kind      string
	EventName string
	GroupBy   string
	Rows      []domain.NameCount
}

func NewHandler(deps Deps) http.Handler {
	tmpl := parseTemplates()
	mux := http.NewServeMux()

	mux.HandleFunc("GET /dashboard/login", handleLoginPage(tmpl))
	mux.HandleFunc("POST /dashboard/login", withLoginRateLimit(deps.LoginLimiter, handleLoginSubmit(deps, tmpl)))
	mux.HandleFunc("POST /dashboard/logout", handleLogout(deps))
	mux.HandleFunc("GET /dashboard", requireSession(deps, handleOverview(deps, tmpl)))
	mux.HandleFunc("GET /dashboard/events/{type}/{name}", requireSession(deps, handleEventDrilldown(deps, tmpl)))
	// Deliberately not wrapped in requireSession: switching projects is an
	// admin-session privilege, independent of which (if any) project's
	// dashboard session the caller currently holds — see handleSwitchProject.
	mux.HandleFunc("POST /dashboard/switch", handleSwitchProject(deps))
	mux.Handle("GET /dashboard/static/", staticHandler())

	return mux
}

func withLoginRateLimit(rl *ratelimit.Limiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !rl.Allow(ratelimit.ClientIP(r)) {
			http.Error(w, "too many login attempts, try again later", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

type contextKey string

const projectIDContextKey contextKey = "traccia.dashboard.project_id"

func requireSession(deps Deps, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID, err := deps.Sessions.SubjectFromRequest(r)
		if err != nil {
			http.Redirect(w, r, "/dashboard/login", http.StatusSeeOther)
			return
		}
		ctx := context.WithValue(r.Context(), projectIDContextKey, projectID)
		next(w, r.WithContext(ctx))
	}
}

func projectIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(projectIDContextKey).(string)
	return id
}

type loginView struct {
	Error string
}

func handleLoginPage(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tmpl.ExecuteTemplate(w, "login-page", loginView{})
	}
}

func handleLoginSubmit(deps Deps, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}

		project, err := deps.Auth.Execute(r.Context(), r.FormValue("api_key"))
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			tmpl.ExecuteTemplate(w, "login-page", loginView{Error: "API key inválida."})
			return
		}

		deps.Sessions.SetCookie(w, project.ID)
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	}
}

func handleLogout(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deps.Sessions.ClearCookie(w)
		http.Redirect(w, r, "/dashboard/login", http.StatusSeeOther)
	}
}

type projectOption struct {
	ID   string
	Name string
}

type overviewView struct {
	RangeDays        int
	ExcludeNamed     bool
	IncludeBots      bool
	Stats            domain.Stats
	TimeseriesJSON   string
	Panels           []PanelView
	CurrentProjectID string
	// Projects is only populated when the request also carries a valid
	// admin session — see Deps.AdminSessions. An empty slice hides the
	// switcher in the template, it never renders a broken one.
	Projects []projectOption
}

func handleOverview(deps Deps, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := projectIDFromContext(r.Context())
		rangeDays := parseRangeDays(r)
		excludeNamed := r.URL.Query().Get("exclude_named") == "true"
		includeBots := r.URL.Query().Get("include_bots") == "true"

		until := time.Now().UTC()
		since := until.AddDate(0, 0, -rangeDays)

		stats, err := deps.GetStats.Execute(r.Context(), usecase.GetStatsInput{
			ProjectID:    projectID,
			Since:        since,
			Until:        until,
			ExcludeNamed: excludeNamed,
			IncludeBots:  includeBots,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		timeseriesJSON, err := json.Marshal(stats.VisitsOverTime)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		view := overviewView{
			RangeDays:        rangeDays,
			ExcludeNamed:     excludeNamed,
			IncludeBots:      includeBots,
			Stats:            stats,
			TimeseriesJSON:   string(timeseriesJSON),
			Panels:           computePanelRows(r.Context(), deps, projectID, since, until),
			CurrentProjectID: projectID,
		}

		templateName := "overview-page"
		if r.Header.Get("HX-Request") == "true" {
			templateName = "overview-fragment"
		} else {
			// Only the full page renders the topbar switcher — skip the
			// extra ListProjects call on every HTMX filter refresh.
			view.Projects = switcherProjects(r, deps)
		}
		if err := tmpl.ExecuteTemplate(w, templateName, view); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

// switcherProjects returns every project for the topbar switcher, but only
// when the request also carries a valid *admin* session — a dashboard
// session alone (a single project's API key) must never be enough to see
// or reach any other project. Returns nil (hides the switcher) whenever
// the admin panel isn't wired up (AdminSessions/ListProjects nil, e.g. a
// deployment using only this package without internal/adapters/admin) or
// the caller isn't an admin.
func switcherProjects(r *http.Request, deps Deps) []projectOption {
	if deps.AdminSessions == nil || deps.ListProjects == nil {
		return nil
	}
	if _, err := deps.AdminSessions.SubjectFromRequest(r); err != nil {
		return nil
	}

	projects, err := deps.ListProjects.Execute(r.Context())
	if err != nil {
		return nil
	}

	options := make([]projectOption, 0, len(projects))
	for _, p := range projects {
		options = append(options, projectOption{ID: p.ID, Name: p.Name})
	}
	return options
}

// handleSwitchProject mints a dashboard session for another project on an
// admin's behalf — deliberately independent of requireSession. The only
// credential that matters here is the admin session, re-verified server
// side rather than trusted from the UI having merely hidden the form: the
// switcher being absent from the rendered page is a convenience, not the
// security boundary.
func handleSwitchProject(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.AdminSessions == nil || deps.GetProject == nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if _, err := deps.AdminSessions.SubjectFromRequest(r); err != nil {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}

		id := r.FormValue("project_id")
		if _, err := deps.GetProject.Execute(r.Context(), id); err != nil {
			http.Error(w, "project not found", http.StatusNotFound)
			return
		}

		deps.Sessions.SetCookie(w, id)
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	}
}

// computePanelRows fills in each plugin panel's Rows for this request. It
// copies Deps.Panels rather than mutating it — that slice is shared across
// every concurrent request, so writing computed data into it directly
// would leak one project's numbers into another's response.
func computePanelRows(ctx context.Context, deps Deps, projectID string, since, until time.Time) []PanelView {
	panels := make([]PanelView, len(deps.Panels))
	copy(panels, deps.Panels)

	if deps.GetMetadataBreakdown == nil {
		return panels
	}

	for i := range panels {
		if panels[i].EventName == "" || panels[i].GroupBy == "" {
			continue
		}
		rows, err := deps.GetMetadataBreakdown.Execute(ctx, usecase.GetMetadataBreakdownInput{
			ProjectID:   projectID,
			Type:        domain.EventTypeCustom,
			EventName:   panels[i].EventName,
			MetadataKey: panels[i].GroupBy,
			Since:       since,
			Until:       until,
		})
		if err == nil {
			panels[i].Rows = rows
		}
	}
	return panels
}

func parseRangeDays(r *http.Request) int {
	switch r.URL.Query().Get("range") {
	case "30":
		return 30
	case "90":
		return 90
	default:
		return 7
	}
}

type eventSampleView struct {
	CreatedAt    string
	VisitorID    string
	MetadataJSON string
}

type drilldownView struct {
	Type    string
	Name    string
	Samples []eventSampleView
}

func handleEventDrilldown(deps Deps, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := projectIDFromContext(r.Context())
		typeParam := r.PathValue("type")
		name := r.PathValue("name")

		var eventType domain.EventType
		switch typeParam {
		case "custom":
			eventType = domain.EventTypeCustom
		case "error":
			eventType = domain.EventTypeError
		default:
			http.Error(w, "unknown event type", http.StatusBadRequest)
			return
		}

		samples, err := deps.GetSamples.Execute(r.Context(), usecase.GetEventSamplesInput{
			ProjectID: projectID,
			Type:      eventType,
			Name:      name,
			Limit:     50,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		views := make([]eventSampleView, 0, len(samples))
		for _, s := range samples {
			metadataJSON, err := json.Marshal(s.Metadata)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			views = append(views, eventSampleView{
				CreatedAt:    s.CreatedAt.Format("2006-01-02 15:04:05"),
				VisitorID:    s.VisitorID,
				MetadataJSON: string(metadataJSON),
			})
		}

		view := drilldownView{Type: typeParam, Name: name, Samples: views}
		if err := tmpl.ExecuteTemplate(w, "event-drilldown", view); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}
