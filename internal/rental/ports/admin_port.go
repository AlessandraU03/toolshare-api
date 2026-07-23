package rentalports

import (
	"context"

	"github.com/google/uuid"
	rentaldomain "github.com/yourusername/tool-inventory-api/internal/rental/domain"
	userdomain "github.com/yourusername/tool-inventory-api/internal/user/domain"
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

// InsuranceClaimOutput indica cuánto le debe el seguro de ToolShare al
// propietario tras ganar una disputa (herramienta con seguro activo), y sus
// datos bancarios registrados para que el administrador haga la
// transferencia manual (no hay API de Mercado Pago para esto).
type InsuranceClaimOutput struct {
	Amount      float64
	BankAccount *userdomain.BankAccount
}

type ResolveDisputeOutput struct {
	Rental *rentaldomain.Rental
	// InsuranceClaim es nil si la herramienta no tenía seguro activo o si
	// la disputa no se resolvió a favor del propietario.
	InsuranceClaim *InsuranceClaimOutput
}

type AdminService interface {
	GetStats(ctx context.Context) (*AdminStatsOutput, error)
	ListRentals(ctx context.Context, status string) ([]*rentaldomain.Rental, error)
	ResolveDispute(ctx context.Context, inp ResolveDisputeInput) (*ResolveDisputeOutput, error)
}
