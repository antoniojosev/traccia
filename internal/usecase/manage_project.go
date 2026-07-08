package usecase

import (
	"context"
	"time"

	"github.com/antoniojosev/traccia/internal/domain"
	"github.com/antoniojosev/traccia/internal/ports"
	"github.com/google/uuid"
)

type CreateProject struct {
	Projects ports.ProjectRepository
	Keys     ports.APIKeyHasher
}

func NewCreateProject(projects ports.ProjectRepository, keys ports.APIKeyHasher) *CreateProject {
	return &CreateProject{Projects: projects, Keys: keys}
}

// Execute creates a project and returns the plaintext API key. That key is
// shown exactly once — only its hash is persisted.
func (uc *CreateProject) Execute(ctx context.Context, name, domainName string) (project domain.Project, plainKey string, err error) {
	plainKey, hash, err := uc.Keys.Generate()
	if err != nil {
		return domain.Project{}, "", err
	}

	project = domain.Project{
		ID:         uuid.NewString(),
		Name:       name,
		Domain:     domainName,
		APIKeyHash: hash,
		CreatedAt:  time.Now().UTC(),
	}

	if err := uc.Projects.Create(ctx, project); err != nil {
		return domain.Project{}, "", err
	}
	return project, plainKey, nil
}

type AuthenticateProject struct {
	Projects ports.ProjectRepository
	Keys     ports.APIKeyHasher
}

func NewAuthenticateProject(projects ports.ProjectRepository, keys ports.APIKeyHasher) *AuthenticateProject {
	return &AuthenticateProject{Projects: projects, Keys: keys}
}

func (uc *AuthenticateProject) Execute(ctx context.Context, plainKey string) (domain.Project, error) {
	return uc.Projects.FindByAPIKeyHash(ctx, uc.Keys.Hash(plainKey))
}
