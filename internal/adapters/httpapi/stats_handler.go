package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/antoniojosev/traccia/internal/usecase"
)

func HandleStats(auth *usecase.AuthenticateProject, stats *usecase.GetStats) http.HandlerFunc {
	return withProjectAuth(auth, func(w http.ResponseWriter, r *http.Request) {
		project, _ := projectFromContext(r.Context())

		in := usecase.GetStatsInput{ProjectID: project.ID}
		if since := r.URL.Query().Get("since"); since != "" {
			if t, err := time.Parse(time.RFC3339, since); err == nil {
				in.Since = t
			}
		}
		if until := r.URL.Query().Get("until"); until != "" {
			if t, err := time.Parse(time.RFC3339, until); err == nil {
				in.Until = t
			}
		}
		in.ExcludeNamed = r.URL.Query().Get("exclude_named") == "true"
		in.IncludeBots = r.URL.Query().Get("include_bots") == "true"

		result, err := stats.Execute(r.Context(), in)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})
}
