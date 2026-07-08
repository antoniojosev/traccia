package httpapi

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"

	"github.com/antoniojosev/traccia/internal/usecase"
)

type createProjectRequest struct {
	Name   string `json:"name"`
	Domain string `json:"domain"`
}

type createProjectResponse struct {
	ProjectID string `json:"project_id"`
	APIKey    string `json:"api_key"`
}

// HandleCreateProject is gated by adminToken, not a per-project API key —
// this is a control-plane action (minting new tenants), not a data-plane
// one. There's no UI for it yet: call it once via curl to bootstrap a
// project, save the returned api_key, it's never shown again.
func HandleCreateProject(adminToken string, createProject *usecase.CreateProject) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if adminToken == "" || subtle.ConstantTimeCompare([]byte(bearerToken(r)), []byte(adminToken)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var req createProjectRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}

		project, plainKey, err := createProject.Execute(r.Context(), req.Name, req.Domain)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(createProjectResponse{ProjectID: project.ID, APIKey: plainKey})
	}
}
