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

// WearLevel define los estados de desgaste de una herramienta.
type WearLevel string

const (
	WearLevelNew       WearLevel = "Nuevo"
	WearLevelGood      WearLevel = "Buen Estado"
	WearLevelWorn      WearLevel = "Desgastado"
)

// RentalStatus define los estados posibles de una orden de renta.
type RentalStatus string

const (
	RentalStatusPendingPayment RentalStatus = "pending_payment"
	RentalStatusFundsHeld      RentalStatus = "funds_held"
	RentalStatusDelivered      RentalStatus = "delivered"
	RentalStatusInUse          RentalStatus = "in_use"
	RentalStatusReturned       RentalStatus = "returned"
	RentalStatusCompleted      RentalStatus = "completed"
	RentalStatusDispute        RentalStatus = "dispute"
	RentalStatusCancelled      RentalStatus = "cancelled"
)

// =============================================================================
// Entidades del dominio — mapean directamente con tablas de PostgreSQL
// =============================================================================

// User representa un usuario del sistema.
// Puede ser Propietario (owner) o Solicitante (requester).
type User struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Password  string    `json:"-"` // Nunca se serializa en respuestas JSON
	Role      Role      `json:"role"`
	Phone     *string   `json:"phone,omitempty"`
	IneNumber *string   `json:"ine_number,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Tool representa una herramienta en el inventario.
// Cada herramienta pertenece a un Propietario (owner).
type Tool struct {
	ID          uuid.UUID  `json:"id"`
	OwnerID     uuid.UUID  `json:"owner_id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Category    string     `json:"category"`
	IsAvailable bool       `json:"is_available"`
	Brand       *string    `json:"brand,omitempty"`
	Model       *string    `json:"model,omitempty"`
	WearLevel   *WearLevel `json:"wear_level,omitempty"`
	Latitude    *float64   `json:"latitude,omitempty"`
	Longitude   *float64   `json:"longitude,omitempty"`
	PricePerDay *float64   `json:"price_per_day,omitempty"`
	PhotoURL    *string    `json:"photo_url,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Rental representa una orden de renta entre un solicitante y una herramienta.
type Rental struct {
	ID                          uuid.UUID    `json:"rental_id"`
	ToolID                      uuid.UUID    `json:"tool_id"`
	RequesterID                 uuid.UUID    `json:"requester_id"`
	Days                        int          `json:"days"`
	PricePerDay                 float64      `json:"price_per_day"`
	Total                       float64      `json:"total"`
	Deposit                     float64      `json:"deposit"`
	Status                      RentalStatus `json:"status"`
	PaymentID                   *string      `json:"payment_id,omitempty"`
	PaymentURL                  *string      `json:"payment_url,omitempty"`
	RequesterConfirmedDelivery  bool         `json:"requester_confirmed_delivery"`
	OwnerConfirmedDelivery      bool         `json:"owner_confirmed_delivery"`
	DeliveryLatitude            *float64     `json:"delivery_latitude,omitempty"`
	DeliveryLongitude           *float64     `json:"delivery_longitude,omitempty"`
	ContractHash                *string      `json:"contract_hash,omitempty"`
	ExpiresAt                   *time.Time   `json:"expires_at,omitempty"`
	DeliveryConfirmedAt         *time.Time   `json:"delivery_confirmed_at,omitempty"`
	ReturnAccepted              *bool        `json:"return_accepted,omitempty"`
	ReturnRejectionReason       *string      `json:"return_rejection_reason,omitempty"`
	FundsReleased               bool         `json:"funds_released"`
	CreatedAt                   time.Time    `json:"created_at"`
	UpdatedAt                   time.Time    `json:"updated_at"`
}

// =============================================================================
// DTOs (Data Transfer Objects) — Structs para request/response de la API
// =============================================================================

// RegisterRequest es el body esperado en POST /api/auth/register.
type RegisterRequest struct {
	Name      string `json:"name"       binding:"required,min=2,max=100"`
	Email     string `json:"email"      binding:"required,email"`
	Password  string `json:"password"   binding:"required,min=8"`
	Role      Role   `json:"role"       binding:"required,oneof=owner requester"`
	Phone     string `json:"phone"`
	IneNumber string `json:"ine_number"`
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
	Name        string     `json:"name"         binding:"required,min=2,max=150"`
	Description string     `json:"description"`
	Category    string     `json:"category"`
	IsAvailable *bool      `json:"is_available"` // Puntero para distinguir false de "no enviado"
	Brand       *string    `json:"brand"`
	Model       *string    `json:"model"`
	WearLevel   *WearLevel `json:"wear_level"`
	Latitude    *float64   `json:"latitude"`
	Longitude   *float64   `json:"longitude"`
	PricePerDay *float64   `json:"price_per_day"`
}

// UpdateToolRequest es el body esperado en PUT /api/tools/:id.
// Todos los campos son opcionales (PATCH semántico sobre PUT).
type UpdateToolRequest struct {
	Name        *string    `json:"name"         binding:"omitempty,min=2,max=150"`
	Description *string    `json:"description"`
	Category    *string    `json:"category"`
	IsAvailable *bool      `json:"is_available"`
	Brand       *string    `json:"brand"`
	Model       *string    `json:"model"`
	WearLevel   *WearLevel `json:"wear_level"`
	Latitude    *float64   `json:"latitude"`
	Longitude   *float64   `json:"longitude"`
	PricePerDay *float64   `json:"price_per_day"`
}

// PhotoUploadResponse es la respuesta del endpoint POST /api/tools/:id/photo.
type PhotoUploadResponse struct {
	PhotoURL string `json:"photo_url"`
}

// SuggestPriceResponse es la respuesta del endpoint GET /api/tools/suggest-price.
type SuggestPriceResponse struct {
	SuggestedPrice float64 `json:"suggested_price"`
	Currency       string  `json:"currency"`
	Unit           string  `json:"unit"`
}

// CreateRentalRequest es el body esperado en POST /api/rentals.
type CreateRentalRequest struct {
	ToolID      uuid.UUID `json:"tool_id"      binding:"required"`
	Days        int       `json:"days"         binding:"required,min=1"`
	PricePerDay float64   `json:"price_per_day" binding:"required,min=0"`
	Total       float64   `json:"total"        binding:"required,min=0"`
	Deposit     float64   `json:"deposit"      binding:"required,min=0"`
}

// CreateRentalResponse es la respuesta del endpoint POST /api/rentals.
type CreateRentalResponse struct {
	RentalID   uuid.UUID    `json:"rental_id"`
	Status     RentalStatus `json:"status"`
	PaymentURL string       `json:"payment_url"`
	ExpiresAt  time.Time    `json:"expires_at"`
}

// ConfirmPaymentRequest es el body para POST /api/rentals/:id/payment.
type ConfirmPaymentRequest struct {
	PaymentID string `json:"payment_id" binding:"required"`
}

// ConfirmPaymentResponse es la respuesta de POST /api/rentals/:id/payment.
type ConfirmPaymentResponse struct {
	RentalID uuid.UUID    `json:"rental_id"`
	Status   RentalStatus `json:"status"`
	Message  string       `json:"message"`
}

// ConfirmDeliveryRequest es el body para POST /api/rentals/:id/confirm-delivery.
type ConfirmDeliveryRequest struct {
	Latitude  float64 `json:"latitude"  binding:"required"`
	Longitude float64 `json:"longitude" binding:"required"`
	Role      string  `json:"role"      binding:"required,oneof=requester owner"`
}

// ConfirmDeliveryResponse es la respuesta de POST /api/rentals/:id/confirm-delivery.
type ConfirmDeliveryResponse struct {
	RentalID            uuid.UUID    `json:"rental_id"`
	Status              RentalStatus `json:"status"`
	ContractHash        string       `json:"contract_hash,omitempty"`
	DeliveryConfirmedAt *time.Time   `json:"delivery_confirmed_at,omitempty"`
}

// ReturnRequest es el body para POST /api/rentals/:id/return.
type ReturnRequest struct {
	Accepted bool   `json:"accepted"`
	Reason   string `json:"reason"`
}

// ReturnResponse es la respuesta de POST /api/rentals/:id/return.
type ReturnResponse struct {
	RentalID          uuid.UUID    `json:"rental_id"`
	Status            RentalStatus `json:"status"`
	FundsReleased     bool         `json:"funds_released"`
	DepositReturned   bool         `json:"deposit_returned"`
	ArbitrationAlert  bool         `json:"arbitration_alert_sent,omitempty"`
}

// ErrorResponse es la estructura estándar para respuestas de error.
type ErrorResponse struct {
	Error string `json:"error"`
}

// SuccessResponse es la estructura estándar para mensajes de éxito simples.
type SuccessResponse struct {
	Message string `json:"message"`
}
