package usecase

import (
	"context"

	"github.com/antoniojosev/traccia/internal/ports"
)

// DeleteProject permanently removes a project and, via the database's own
// foreign keys, every event and visitor it ever recorded. There's no undo.
type DeleteProject struct {
	Projects ports.ProjectRepository
}

func NewDeleteProject(projects ports.ProjectRepository) *DeleteProject {
	return &DeleteProject{Projects: projects}
}

func (uc *DeleteProject) Execute(ctx context.Context, id string) error {
	return uc.Projects.Delete(ctx, id)
}
