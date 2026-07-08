package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/antoniojosev/traccia/internal/domain"
	"github.com/antoniojosev/traccia/internal/usecase"
)

type trackRequest struct {
	ProjectID string         `json:"project_id"`
	VisitorID string         `json:"visitor_id"`
	Type      string         `json:"type"`
	Name      string         `json:"name"`
	Path      string         `json:"path"`
	Referrer  string         `json:"referrer"`
	Metadata  map[string]any `json:"metadata"`
}

// HandleTrack is identified by the public project_id in the payload, not a
// secret — the same tradeoff GA/Plausible/Umami make: anyone who can read
// your page source can already send fake events to your project_id, so
// gating this behind a "secret" would be theater. What must stay secret is
// the ability to *read* stats, which is a separate, key-protected endpoint.
// Referential integrity (an unknown project_id fails to insert) comes for
// free from the events.project_id foreign key, so no repository lookup is
// needed on this hot path.
func HandleTrack(track *usecase.TrackEvent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req trackRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}

		err := track.Execute(r.Context(), usecase.TrackEventInput{
			ProjectID: req.ProjectID,
			VisitorID: req.VisitorID,
			Type:      domain.EventType(req.Type),
			Name:      req.Name,
			Path:      req.Path,
			Referrer:  req.Referrer,
			IP:        clientIP(r),
			UserAgent: r.UserAgent(),
			Metadata:  req.Metadata,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusAccepted)
	}
}

type identifyRequest struct {
	ProjectID  string         `json:"project_id"`
	VisitorID  string         `json:"visitor_id"`
	Name       string         `json:"name"`
	Properties map[string]any `json:"properties"`
}

// HandleIdentify is public the same way HandleTrack is — see its comment.
func HandleIdentify(identify *usecase.IdentifyVisitor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req identifyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}

		err := identify.Execute(r.Context(), usecase.IdentifyVisitorInput{
			ProjectID:  req.ProjectID,
			VisitorID:  req.VisitorID,
			Name:       req.Name,
			Properties: req.Properties,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusAccepted)
	}
}

// clientIP trusts X-Forwarded-For only because Traccia is expected to run
// behind a reverse proxy you control (Caddy/nginx/Dokploy) that sets it —
// if you expose the app directly to the internet, this header is spoofable.
func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	host := r.RemoteAddr
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		return host[:idx]
	}
	return host
}
