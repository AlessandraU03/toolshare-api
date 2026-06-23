package userhandler

import (
	"time"

	userdomain "github.com/yourusername/tool-inventory-api/internal/user/domain"
	userports "github.com/yourusername/tool-inventory-api/internal/user/ports"
)

type RegisterRequest struct {
	Name     string          `json:"name"     binding:"required,min=2,max=100"  example:"María López"`
	Email    string          `json:"email"    binding:"required,email"           example:"maria@ejemplo.com"`
	Password string          `json:"password" binding:"required,min=8"           example:"segura123"`
	Role     userdomain.Role `json:"role"     binding:"required,oneof=owner requester" example:"owner"`
	Phone    string          `json:"phone"    binding:"required"                 example:"5512345678"`
	INE      string          `json:"ine"      binding:"required"                 example:"PRRLSS85010212H700"`
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
	IsPro     bool   `json:"is_pro"`
	Phone     string `json:"phone"`
	INE       string `json:"ine"`
	CreatedAt string `json:"created_at"`
}

type AuthResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

func ToAuthResponse(out *userports.AuthOutput) AuthResponse {
	return AuthResponse{
		Token: out.Token,
		User:  ToUserResponse(out.User),
	}
}

func ToUserResponse(u *userdomain.User) UserResponse {
	return UserResponse{
		ID:        u.ID.String(),
		Name:      u.Name,
		Email:     u.Email,
		Role:      string(u.Role),
		IsPro:     u.IsPro,
		Phone:     u.Phone,
		INE:       u.INE,
		CreatedAt: u.CreatedAt.Format(time.RFC3339),
	}
}
