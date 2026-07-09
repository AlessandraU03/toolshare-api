package rentalports

import (
	"context"
	"time"

	"github.com/google/uuid"
	rentaldomain "github.com/yourusername/tool-inventory-api/internal/rental/domain"
	sharedports "github.com/yourusername/tool-inventory-api/internal/shared/ports"
)

type CreateRentalInput struct {
	ToolID        uuid.UUID
	RequesterID   uuid.UUID
	StartDate     time.Time
	EndDate       time.Time
	PaymentMethod string // "card" o "cash"
	CardToken     string // opcional; si se envía, se congela el pago en MP
	PayerEmail    string // requerido cuando CardToken está presente
	IPAddress     string
	DeviceID      string
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

	VerifyContract(ctx context.Context, rentalID uuid.UUID) (bool, string, string, error)
	LogUserFingerprint(ctx context.Context, userID uuid.UUID, ipAddress, deviceID string) error

	// CreatePreference crea una preferencia de Checkout Pro en MP y devuelve el init_point.
	CreatePreference(ctx context.Context, rentalID uuid.UUID, requesterID uuid.UUID, payerEmail string) (sharedports.CreatePreferenceOutput, error)
	// UpdatePaymentStatus actualiza el payment_id y status de una renta (llamado desde el webhook).
	UpdatePaymentStatus(ctx context.Context, rentalID uuid.UUID, paymentID, status string) error

	GetMessages(ctx context.Context, rentalID uuid.UUID, userID uuid.UUID) ([]*rentaldomain.Message, error)
	SendMessage(ctx context.Context, inp SendMessageInput) (*rentaldomain.Message, error)
}
