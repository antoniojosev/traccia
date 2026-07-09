package usecase

import (
	"context"
	"errors"

	"github.com/antoniojosev/traccia/internal/domain"
	"github.com/antoniojosev/traccia/internal/ports"
)

var ErrProjectNameRequired = errors.New("usecase: project name is required")

type UpdateProject struct {
	Projects ports.ProjectRepository
}

func NewUpdateProject(projects ports.ProjectRepository) *UpdateProject {
	return &UpdateProject{Projects: projects}
}

func (uc *UpdateProject) Execute(ctx context.Context, id, name, domainName string) (domain.Project, error) {
	if name == "" {
		return domain.Project{}, ErrProjectNameRequired
	}

	project, err := uc.Projects.FindByID(ctx, id)
	if err != nil {
		return domain.Project{}, err
	}

	project.Name = name
	project.Domain = domainName
	if err := uc.Projects.Update(ctx, project); err != nil {
		return domain.Project{}, err
	}
	return project, nil
}
