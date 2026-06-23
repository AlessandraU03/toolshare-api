package rentalports

import (
	"context"
	"github.com/google/uuid"
	rentaldomain "github.com/yourusername/tool-inventory-api/internal/rental/domain"
)

type RentalRepository interface {
	Create(ctx context.Context, rental *rentaldomain.Rental) (*rentaldomain.Rental, error)
	FindByID(ctx context.Context, id uuid.UUID) (*rentaldomain.Rental, error)
	FindByUser(ctx context.Context, userID uuid.UUID) ([]*rentaldomain.Rental, error)
	Update(ctx context.Context, rental *rentaldomain.Rental) (*rentaldomain.Rental, error)
}
