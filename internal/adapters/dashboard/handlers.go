package dashboard

import (
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"time"

	"github.com/antoniojosev/traccia/internal/adapters/session"
	"github.com/antoniojosev/traccia/internal/domain"
	"github.com/antoniojosev/traccia/internal/usecase"
)

type Deps struct {
	Auth       *usecase.AuthenticateProject
	GetStats   *usecase.GetStats
	GetSamples *usecase.GetEventSamples
	Sessions   *session.Manager
	// Panels are plugin-declared (see plugins.Manager.Panels), rendered
	// here in the core so a plugin never ships its own frontend JS — the
	// reason this dashboard is server-rendered HTMX instead of a SPA.
	Panels []PanelView
}

type PanelView struct {
	Title string
	Kind  string
}

func NewHandler(deps Deps) http.Handler {
	tmpl := parseTemplates()
	mux := http.NewServeMux()

	mux.HandleFunc("GET /dashboard/login", handleLoginPage(tmpl))
	mux.HandleFunc("POST /dashboard/login", handleLoginSubmit(deps, tmpl))
	mux.HandleFunc("POST /dashboard/logout", handleLogout(deps))
	mux.HandleFunc("GET /dashboard", requireSession(deps, handleOverview(deps, tmpl)))
	mux.HandleFunc("GET /dashboard/events/{type}/{name}", requireSession(deps, handleEventDrilldown(deps, tmpl)))
	mux.Handle("GET /dashboard/static/", staticHandler())

	return mux
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

type overviewView struct {
	RangeDays      int
	ExcludeNamed   bool
	IncludeBots    bool
	Stats          domain.Stats
	TimeseriesJSON string
	Panels         []PanelView
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
			RangeDays:      rangeDays,
			ExcludeNamed:   excludeNamed,
			IncludeBots:    includeBots,
			Stats:          stats,
			TimeseriesJSON: string(timeseriesJSON),
			Panels:         deps.Panels,
		}

		templateName := "overview-page"
		if r.Header.Get("HX-Request") == "true" {
			templateName = "overview-fragment"
		}
		if err := tmpl.ExecuteTemplate(w, templateName, view); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
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
