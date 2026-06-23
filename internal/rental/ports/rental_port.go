package rentalports

import (
	"context"
	"time"

	"github.com/google/uuid"
	rentaldomain "github.com/yourusername/tool-inventory-api/internal/rental/domain"
)

type CreateRentalInput struct {
	ToolID      uuid.UUID
	RequesterID uuid.UUID
	StartDate   time.Time
	EndDate     time.Time
	CardToken   string // opcional; si se envía, se congela el pago en MP
	PayerEmail  string // requerido cuando CardToken está presente
}

type RentalService interface {
	Create(ctx context.Context, inp CreateRentalInput) (*rentaldomain.Rental, error)
	GetByID(ctx context.Context, id uuid.UUID) (*rentaldomain.Rental, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]*rentaldomain.Rental, error)

	// ConfirmDelivery: apretón de manos en la entrega.
	// lat/lng son opcionales (0,0 si no se envían).
	// Cuando ambas partes confirman → status "active" + contrato SHA-256 generado.
	ConfirmDelivery(ctx context.Context, rentalID uuid.UUID, userID uuid.UUID, lat, lng float64) (*rentaldomain.Rental, error)

	// ConfirmReturn: apretón de manos en la devolución.
	// Cuando ambas partes confirman → status "completed" + captura de pago en MP.
	ConfirmReturn(ctx context.Context, rentalID uuid.UUID, userID uuid.UUID) (*rentaldomain.Rental, error)

	// Dispute: el propietario reporta daño en la devolución.
	// status → "disputed", MP captura el depósito como penalización.
	Dispute(ctx context.Context, rentalID uuid.UUID, ownerID uuid.UUID, reason string) (*rentaldomain.Rental, error)

	Cancel(ctx context.Context, rentalID uuid.UUID, userID uuid.UUID) (*rentaldomain.Rental, error)
}
