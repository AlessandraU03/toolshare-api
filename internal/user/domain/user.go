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

// MPSellerAccount representa la cuenta de Mercado Pago que un propietario
// vinculó vía OAuth (Marketplace) para recibir directamente su parte de cada
// pago. Si SellerUserID está vacío, el propietario no ha conectado su cuenta.
type MPSellerAccount struct {
	SellerUserID string
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

// Connected indica si el propietario ya vinculó una cuenta de Mercado Pago.
func (a *MPSellerAccount) Connected() bool {
	return a != nil && a.SellerUserID != ""
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
