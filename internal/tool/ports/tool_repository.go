package toolports

import (
	"context"
	"github.com/google/uuid"
	tooldomain "github.com/yourusername/tool-inventory-api/internal/tool/domain"
)

type ToolFilter struct {
	OnlyAvailable bool
	Category      string
	Search        string
	OwnerID       *uuid.UUID
}

type ToolRepository interface {
	Create(ctx context.Context, tool *tooldomain.Tool) (*tooldomain.Tool, error)
	FindByID(ctx context.Context, id uuid.UUID) (*tooldomain.Tool, error)
	FindAll(ctx context.Context, filter ToolFilter) ([]*tooldomain.Tool, error)
	Update(ctx context.Context, tool *tooldomain.Tool) (*tooldomain.Tool, error)
	Delete(ctx context.Context, id uuid.UUID) error
	SetAvailability(ctx context.Context, id uuid.UUID, available bool) error
}
