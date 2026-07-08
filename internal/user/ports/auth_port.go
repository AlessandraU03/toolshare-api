package userports

import (
	"context"

	"github.com/google/uuid"
	sharedports "github.com/yourusername/tool-inventory-api/internal/shared/ports"
	userdomain "github.com/yourusername/tool-inventory-api/internal/user/domain"
)

type RegisterInput struct {
	Name     string
	Email    string
	Password string
	Role     userdomain.Role
	Phone    string
	INE      string
}

type AuthOutput struct {
	Token string
	User  *userdomain.User
}

type AuthService interface {
	Register(ctx context.Context, inp RegisterInput) (*AuthOutput, error)
	Login(ctx context.Context, email, password string) (*AuthOutput, error)

	// CreateSubscriptionPreference crea una preferencia de Checkout Pro en MP
	// para que el usuario pague la suscripción Pro mensual.
	CreateSubscriptionPreference(ctx context.Context, userID uuid.UUID) (sharedports.CreatePreferenceOutput, error)

	// GetProfile devuelve el usuario actual (usado para refrescar is_pro tras el pago).
	GetProfile(ctx context.Context, userID uuid.UUID) (*userdomain.User, error)

	// ConfirmSubscriptionPayment verifica el pago directamente contra MP y activa el plan Pro.
	ConfirmSubscriptionPayment(ctx context.Context, userID uuid.UUID, paymentID string) error
}
