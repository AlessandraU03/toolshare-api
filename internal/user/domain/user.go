package userdomain

import (
	"github.com/google/uuid"
	"time"
)

type Role string

const (
	RoleOwner     Role = "owner"
	RoleRequester Role = "requester"
	RoleAdmin     Role = "admin"
)

type User struct {
	ID           uuid.UUID
	Name         string
	Email        string
	Password     string
	Role         Role
	IsPro        bool
	Phone        string
	INE          string
	MPCustomerID string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// SavedCard es una tarjeta guardada por el usuario en su Customer de Mercado
// Pago, para pagar rentas futuras sin volver a capturar los datos completos.
type SavedCard struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	MPCardID        string
	CardBrand       string
	LastFourDigits  string
	ExpirationMonth int
	ExpirationYear  int
	CreatedAt       time.Time
}
