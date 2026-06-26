package rentalports

import (
	"context"
	"time"

	"github.com/google/uuid"
	rentaldomain "github.com/yourusername/tool-inventory-api/internal/rental/domain"
)

type CreateRentalInput struct {
	ToolID        uuid.UUID
	RequesterID   uuid.UUID
	StartDate     time.Time
	EndDate       time.Time
	PaymentMethod string // "card" o "cash"
	CardToken     string // opcional; si se envía, se congela el pago en MP
	PayerEmail    string // requerido cuando CardToken está presente
}

type SendMessageInput struct {
	RentalID uuid.UUID
	SenderID uuid.UUID
	Message  string
}

type RentalService interface {
	Create(ctx context.Context, inp CreateRentalInput) (*rentaldomain.Rental, error)
	GetByID(ctx context.Context, id uuid.UUID) (*rentaldomain.Rental, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]*rentaldomain.Rental, error)

	ConfirmDelivery(ctx context.Context, rentalID uuid.UUID, userID uuid.UUID, lat, lng float64) (*rentaldomain.Rental, error)
	ConfirmReturn(ctx context.Context, rentalID uuid.UUID, userID uuid.UUID) (*rentaldomain.Rental, error)
	Dispute(ctx context.Context, rentalID uuid.UUID, ownerID uuid.UUID, reason string) (*rentaldomain.Rental, error)
	Cancel(ctx context.Context, rentalID uuid.UUID, userID uuid.UUID) (*rentaldomain.Rental, error)

	GetMessages(ctx context.Context, rentalID uuid.UUID, userID uuid.UUID) ([]*rentaldomain.Message, error)
	SendMessage(ctx context.Context, inp SendMessageInput) (*rentaldomain.Message, error)
}
