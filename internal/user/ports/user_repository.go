package userports

import (
	"context"
	"github.com/google/uuid"
	userdomain "github.com/yourusername/tool-inventory-api/internal/user/domain"
)

type UserRepository interface {
	Create(ctx context.Context, user *userdomain.User) (*userdomain.User, error)
	FindByEmail(ctx context.Context, email string) (*userdomain.User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*userdomain.User, error)
	UpdateIsPro(ctx context.Context, id uuid.UUID, isPro bool) error

	// GetMPCustomerID devuelve el Customer de Mercado Pago del usuario, o "" si aún no tiene uno.
	GetMPCustomerID(ctx context.Context, id uuid.UUID) (string, error)
	// SetMPCustomerID guarda el Customer de Mercado Pago creado para el usuario.
	SetMPCustomerID(ctx context.Context, id uuid.UUID, customerID string) error

	SaveCard(ctx context.Context, card *userdomain.SavedCard) (*userdomain.SavedCard, error)
	ListCards(ctx context.Context, userID uuid.UUID) ([]*userdomain.SavedCard, error)
	DeleteCard(ctx context.Context, userID uuid.UUID, cardID uuid.UUID) (mpCardID string, err error)
}
