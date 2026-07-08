package httpapi

import (
	"net/http"

	traccia "github.com/antoniojosev/traccia"
	"github.com/antoniojosev/traccia/internal/usecase"
)

type Deps struct {
	AdminToken      string
	Auth            *usecase.AuthenticateProject
	CreateProject   *usecase.CreateProject
	TrackEvent      *usecase.TrackEvent
	IdentifyVisitor *usecase.IdentifyVisitor
	GetStats        *usecase.GetStats
}

func NewRouter(deps Deps) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/v1/track", withCORS(HandleTrack(deps.TrackEvent)))
	mux.HandleFunc("POST /api/v1/identify", withCORS(HandleIdentify(deps.IdentifyVisitor)))
	mux.HandleFunc("GET /api/v1/stats", withCORS(HandleStats(deps.Auth, deps.GetStats)))
	mux.HandleFunc("POST /api/v1/projects", HandleCreateProject(deps.AdminToken, deps.CreateProject))

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("GET /t.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.Write(traccia.TrackingScript)
	})

	return mux
}

// withCORS allows any origin on the tracking/stats endpoints — Traccia is
// meant to receive events from whatever domain the SDK is embedded on,
// which isn't known in advance (same reason Google Analytics/Plausible/
// Umami all set Access-Control-Allow-Origin: * on ingest).
func withCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Api-Key")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}
