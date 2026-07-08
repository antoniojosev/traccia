// Package httpapi is Traccia's default HTTP transport adapter: it wires
// incoming requests to usecases and nothing else. No business logic lives
// here — that keeps usecases testable without an HTTP server.
package httpapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/antoniojosev/traccia/internal/domain"
	"github.com/antoniojosev/traccia/internal/usecase"
)

type contextKey string

const projectContextKey contextKey = "traccia.project"

// withProjectAuth resolves the project from the "Authorization: Bearer <api-key>"
// header (or an "api-key" header, so the SDK's sendBeacon calls — which
// can't set arbitrary headers cross-origin without a preflight — can pass
// it as a query param instead) and stores it in the request context.
func withProjectAuth(auth *usecase.AuthenticateProject, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := bearerToken(r)
		if key == "" {
			http.Error(w, "missing api key", http.StatusUnauthorized)
			return
		}

		project, err := auth.Execute(r.Context(), key)
		if err != nil {
			http.Error(w, "invalid api key", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), projectContextKey, project)
		next(w, r.WithContext(ctx))
	}
}

func bearerToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	if k := r.Header.Get("Api-Key"); k != "" {
		return k
	}
	return r.URL.Query().Get("api_key")
}

func projectFromContext(ctx context.Context) (domain.Project, bool) {
	p, ok := ctx.Value(projectContextKey).(domain.Project)
	return p, ok
}
