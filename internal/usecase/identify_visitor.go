package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/antoniojosev/traccia/internal/domain"
	"github.com/antoniojosev/traccia/internal/ports"
)

var ErrInvalidIdentify = errors.New("traccia: invalid identify call")

type IdentifyVisitorInput struct {
	ProjectID  string
	VisitorID  string
	Name       string
	Properties map[string]any
}

type IdentifyVisitor struct {
	Visitors ports.VisitorRepository
}

func NewIdentifyVisitor(visitors ports.VisitorRepository) *IdentifyVisitor {
	return &IdentifyVisitor{Visitors: visitors}
}

func (uc *IdentifyVisitor) Execute(ctx context.Context, in IdentifyVisitorInput) error {
	if in.ProjectID == "" || in.VisitorID == "" {
		return ErrInvalidIdentify
	}
	return uc.Visitors.Upsert(ctx, domain.Visitor{
		ProjectID:  in.ProjectID,
		VisitorID:  in.VisitorID,
		Name:       in.Name,
		Properties: in.Properties,
		UpdatedAt:  time.Now().UTC(),
	})
}
