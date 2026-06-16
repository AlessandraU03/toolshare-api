package dto

import (
	"time"

	"github.com/yourusername/tool-inventory-api/internal/core/domain"
	"github.com/yourusername/tool-inventory-api/internal/core/ports/input"
)

type RegisterRequest struct {
	Name     string      `json:"name"     binding:"required,min=2,max=100"  example:"María López"`
	Email    string      `json:"email"    binding:"required,email"           example:"maria@ejemplo.com"`
	Password string      `json:"password" binding:"required,min=8"           example:"segura123"`
	Role     domain.Role `json:"role"     binding:"required,oneof=owner requester" example:"owner"`
}

type LoginRequest struct {
	Email    string `json:"email"    binding:"required,email" example:"maria@ejemplo.com"`
	Password string `json:"password" binding:"required"       example:"segura123"`
}

type UserResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
}

type AuthResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

func ToAuthResponse(out *input.AuthOutput) AuthResponse {
	return AuthResponse{
		Token: out.Token,
		User:  ToUserResponse(out.User),
	}
}

func ToUserResponse(u *domain.User) UserResponse {
	return UserResponse{
		ID:        u.ID.String(),
		Name:      u.Name,
		Email:     u.Email,
		Role:      string(u.Role),
		CreatedAt: u.CreatedAt.Format(time.RFC3339),
	}
}
