package usecase

import (
	"context"

	"github.com/antoniojosev/traccia/internal/domain"
	"github.com/antoniojosev/traccia/internal/ports"
)

// GetProject looks a project up by its own ID — distinct from
// AuthenticateProject, which looks one up by (hash of) its secret API key.
// The admin panel uses this to validate a project exists before minting it
// a dashboard session, since the admin never handles that project's key.
type GetProject struct {
	Projects ports.ProjectRepository
}

func NewGetProject(projects ports.ProjectRepository) *GetProject {
	return &GetProject{Projects: projects}
}

func (uc *GetProject) Execute(ctx context.Context, id string) (domain.Project, error) {
	return uc.Projects.FindByID(ctx, id)
}
