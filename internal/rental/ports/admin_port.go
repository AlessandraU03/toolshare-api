package rentalports

import (
	"context"

	"github.com/google/uuid"
	rentaldomain "github.com/yourusername/tool-inventory-api/internal/rental/domain"
)

type AdminStatsOutput struct {
	TotalTools      int     `json:"total_tools"`
	TotalRentals    int     `json:"total_rentals"`
	ActiveRentals   int     `json:"active_rentals"`
	DisputedRentals int     `json:"disputed_rentals"`
	FrozenFunds     float64 `json:"frozen_funds"`
}

type ResolveDisputeInput struct {
	RentalID uuid.UUID `json:"rental_id"`
	Action   string    `json:"action" binding:"required,oneof=capture refund"` // capture: cobrar garantía a favor de owner, refund: reembolsar a solicitante
	Notes    string    `json:"notes"`
}

type AdminService interface {
	GetStats(ctx context.Context) (*AdminStatsOutput, error)
	ListRentals(ctx context.Context, status string) ([]*rentaldomain.Rental, error)
	ResolveDispute(ctx context.Context, inp ResolveDisputeInput) (*rentaldomain.Rental, error)
}
