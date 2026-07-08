package usecase

import (
	"context"

	"github.com/antoniojosev/traccia/internal/domain"
	"github.com/antoniojosev/traccia/internal/ports"
)

type ListProjects struct {
	Projects ports.ProjectRepository
}

func NewListProjects(projects ports.ProjectRepository) *ListProjects {
	return &ListProjects{Projects: projects}
}

func (uc *ListProjects) Execute(ctx context.Context) ([]domain.Project, error) {
	return uc.Projects.List(ctx)
}
