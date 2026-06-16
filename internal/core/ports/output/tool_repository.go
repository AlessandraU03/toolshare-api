package output

import (
	"context"

	"github.com/google/uuid"
	"github.com/yourusername/tool-inventory-api/internal/core/domain"
)

type ToolFilter struct {
	OnlyAvailable bool
	Category      string
	Search        string
	OwnerID       *uuid.UUID
}

type ToolRepository interface {
	Create(ctx context.Context, tool *domain.Tool) (*domain.Tool, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Tool, error)
	FindAll(ctx context.Context, filter ToolFilter) ([]*domain.Tool, error)
	Update(ctx context.Context, tool *domain.Tool) (*domain.Tool, error)
	Delete(ctx context.Context, id uuid.UUID) error
	SetAvailability(ctx context.Context, id uuid.UUID, available bool) error
}
