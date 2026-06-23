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
}
