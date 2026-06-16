package dto

import (
	"time"

	"github.com/yourusername/tool-inventory-api/internal/core/domain"
)

// ── Request ───────────────────────────────────────────────────────────────────

type CreateRentalRequest struct {
	ToolID     string `json:"tool_id"    binding:"required,uuid" example:"550e8400-e29b-41d4-a716-446655440000"`
	StartDate  string `json:"start_date" binding:"required"      example:"2026-06-20T09:00:00Z"`
	EndDate    string `json:"end_date"   binding:"required"      example:"2026-06-25T18:00:00Z"`
	CardToken  string `json:"card_token"  example:"TEST-card-token"` // opcional; generado por el SDK de MP en el frontend
	PayerEmail string `json:"payer_email" example:"solicitante@ejemplo.com"` // requerido si card_token está presente
}

// ConfirmDeliveryRequest permite enviar coordenadas GPS al confirmar entrega.
// Los campos son opcionales; si se omiten el contrato se genera con lat/lng=0.
type ConfirmDeliveryRequest struct {
	Latitude  float64 `json:"latitude"  example:"19.432608"`
	Longitude float64 `json:"longitude" example:"-99.133209"`
}

type DisputeRequest struct {
	Reason string `json:"reason" binding:"required,min=10" example:"La herramienta llegó con el motor quemado"`
}

// ── Response ──────────────────────────────────────────────────────────────────

type RentalResponse struct {
	ID          string  `json:"id"`
	ToolID      string  `json:"tool_id"`
	RequesterID string  `json:"requester_id"`
	OwnerID     string  `json:"owner_id"`
	StartDate   string  `json:"start_date"`
	EndDate     string  `json:"end_date"`
	DailyRate   float64 `json:"daily_rate"`
	TotalAmount float64 `json:"total_amount"`
	Status      string  `json:"status"`

	// Pago
	MPPaymentID      string  `json:"mp_payment_id,omitempty"`
	PaymentStatus    string  `json:"payment_status,omitempty"`
	DeductibleAmount float64 `json:"deductible_amount"`

	// Apretón de manos: entrega
	OwnerConfirmedDelivery     bool `json:"owner_confirmed_delivery"`
	RequesterConfirmedDelivery bool `json:"requester_confirmed_delivery"`

	// Contrato digital
	ContractHash string  `json:"contract_hash,omitempty"`
	DeliveryLat  float64 `json:"delivery_lat,omitempty"`
	DeliveryLng  float64 `json:"delivery_lng,omitempty"`
	DeliveryAt   string  `json:"delivery_at,omitempty"`

	// Apretón de manos: devolución
	RequesterConfirmedReturn bool `json:"requester_confirmed_return"`
	OwnerConfirmedReturn     bool `json:"owner_confirmed_return"`

	// Disputa
	DisputeReason string `json:"dispute_reason,omitempty"`

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func ToRentalResponse(r *domain.Rental) RentalResponse {
	resp := RentalResponse{
		ID:                         r.ID.String(),
		ToolID:                     r.ToolID.String(),
		RequesterID:                r.RequesterID.String(),
		OwnerID:                    r.OwnerID.String(),
		StartDate:                  r.StartDate.Format(time.RFC3339),
		EndDate:                    r.EndDate.Format(time.RFC3339),
		DailyRate:                  r.DailyRate,
		TotalAmount:                r.TotalAmount,
		Status:                     string(r.Status),
		MPPaymentID:                r.MPPaymentID,
		PaymentStatus:              r.PaymentStatus,
		DeductibleAmount:           r.DeductibleAmount,
		OwnerConfirmedDelivery:     r.OwnerConfirmedDelivery,
		RequesterConfirmedDelivery: r.RequesterConfirmedDelivery,
		ContractHash:               r.ContractHash,
		DeliveryLat:                r.DeliveryLat,
		DeliveryLng:                r.DeliveryLng,
		RequesterConfirmedReturn:   r.RequesterConfirmedReturn,
		OwnerConfirmedReturn:       r.OwnerConfirmedReturn,
		DisputeReason:              r.DisputeReason,
		CreatedAt:                  r.CreatedAt.Format(time.RFC3339),
		UpdatedAt:                  r.UpdatedAt.Format(time.RFC3339),
	}
	if r.DeliveryAt != nil {
		resp.DeliveryAt = r.DeliveryAt.Format(time.RFC3339)
	}
	return resp
}

func ToRentalListResponse(rentals []*domain.Rental) []RentalResponse {
	resp := make([]RentalResponse, 0, len(rentals))
	for _, r := range rentals {
		resp = append(resp, ToRentalResponse(r))
	}
	return resp
}
