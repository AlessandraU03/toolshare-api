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
	FindAll(ctx context.Context, status string) ([]*rentaldomain.Rental, error)
	GetAdminStats(ctx context.Context) (int, int, int, float64, error)
	GetMessages(ctx context.Context, rentalID uuid.UUID) ([]*rentaldomain.Message, error)
	CreateMessage(ctx context.Context, msg *rentaldomain.Message) (*rentaldomain.Message, error)
}
