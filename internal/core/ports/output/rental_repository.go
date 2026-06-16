package output

import (
	"context"

	"github.com/google/uuid"
	"github.com/yourusername/tool-inventory-api/internal/core/domain"
)

type RentalRepository interface {
	Create(ctx context.Context, rental *domain.Rental) (*domain.Rental, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Rental, error)
	FindByUser(ctx context.Context, userID uuid.UUID) ([]*domain.Rental, error)
	Update(ctx context.Context, rental *domain.Rental) (*domain.Rental, error)
}
