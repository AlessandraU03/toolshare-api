package input

import (
	"context"

	"github.com/yourusername/tool-inventory-api/internal/core/domain"
)

type RegisterInput struct {
	Name     string
	Email    string
	Password string
	Role     domain.Role
}

type AuthOutput struct {
	Token string
	User  *domain.User
}

type AuthService interface {
	Register(ctx context.Context, inp RegisterInput) (*AuthOutput, error)
	Login(ctx context.Context, email, password string) (*AuthOutput, error)
}
