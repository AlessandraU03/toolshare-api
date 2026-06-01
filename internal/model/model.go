// Package model define las estructuras de datos que mapean
// directamente con las tablas de PostgreSQL.
package model

import (
	"time"

	"github.com/google/uuid"
)

// Role representa los tipos de usuario permitidos en el sistema.
type Role string

const (
	RoleOwner     Role = "owner"
	RoleRequester Role = "requester"
)

// User representa un usuario del sistema.
// Puede ser Propietario (owner) o Solicitante (requester).
type User struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Password  string    `json:"-"` // Nunca se serializa en respuestas JSON
	Role      Role      `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Tool representa una herramienta en el inventario.
// Cada herramienta pertenece a un Propietario (owner).
type Tool struct {
	ID          uuid.UUID `json:"id"`
	OwnerID     uuid.UUID `json:"owner_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	IsAvailable bool      `json:"is_available"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// =============================================================================
// DTOs (Data Transfer Objects) — Structs para request/response de la API
// =============================================================================

// RegisterRequest es el body esperado en POST /api/auth/register.
type RegisterRequest struct {
	Name     string `json:"name"     binding:"required,min=2,max=100"`
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	Role     Role   `json:"role"     binding:"required,oneof=owner requester"`
}

// LoginRequest es el body esperado en POST /api/auth/login.
type LoginRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// AuthResponse es la respuesta devuelta tras un login o registro exitoso.
type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

// CreateToolRequest es el body esperado en POST /api/tools.
type CreateToolRequest struct {
	Name        string `json:"name"        binding:"required,min=2,max=150"`
	Description string `json:"description"`
	Category    string `json:"category"`
	IsAvailable *bool  `json:"is_available"` // Puntero para distinguir false de "no enviado"
}

// UpdateToolRequest es el body esperado en PUT /api/tools/:id.
// Todos los campos son opcionales (PATCH semántico sobre PUT).
type UpdateToolRequest struct {
	Name        *string `json:"name"        binding:"omitempty,min=2,max=150"`
	Description *string `json:"description"`
	Category    *string `json:"category"`
	IsAvailable *bool   `json:"is_available"`
}

// ErrorResponse es la estructura estándar para respuestas de error.
type ErrorResponse struct {
	Error string `json:"error"`
}

// SuccessResponse es la estructura estándar para mensajes de éxito simples.
type SuccessResponse struct {
	Message string `json:"message"`
}
