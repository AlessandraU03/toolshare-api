package userports

import (
	"context"
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
}
