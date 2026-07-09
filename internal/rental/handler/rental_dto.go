package rentalhandler

import (
	"time"
	rentaldomain "github.com/yourusername/tool-inventory-api/internal/rental/domain"
)

// ── Request ───────────────────────────────────────────────────────────────────

type CreateRentalRequest struct {
	ToolID        string `json:"tool_id"        binding:"required,uuid"`
	StartDate     string `json:"start_date"     binding:"required"`
	EndDate       string `json:"end_date"       binding:"required"`
	PaymentMethod string `json:"payment_method"` // "card" o "cash"
	CardToken     string `json:"card_token"`
	PayerEmail    string `json:"payer_email"`
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
	ID            string  `json:"id"`
	ToolID        string  `json:"tool_id"`
	RequesterID   string  `json:"requester_id"`
	OwnerID       string  `json:"owner_id"`
	OwnerName     string  `json:"owner_name"`
	RequesterName string  `json:"requester_name"`
	StartDate     string  `json:"start_date"`
	EndDate     string  `json:"end_date"`
	DailyRate   float64 `json:"daily_rate"`
	TotalAmount float64 `json:"total_amount"`
	Status        string  `json:"status"`
	PaymentMethod string  `json:"payment_method"`

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

func ToRentalResponse(r *rentaldomain.Rental) RentalResponse {
	resp := RentalResponse{
		ID:                         r.ID.String(),
		ToolID:                     r.ToolID.String(),
		RequesterID:                r.RequesterID.String(),
		OwnerID:                    r.OwnerID.String(),
		OwnerName:                  r.OwnerName,
		RequesterName:              r.RequesterName,
		StartDate:                  r.StartDate.Format(time.RFC3339),
		EndDate:                    r.EndDate.Format(time.RFC3339),
		DailyRate:                  r.DailyRate,
		TotalAmount:                r.TotalAmount,
		Status:                     string(r.Status),
		PaymentMethod:              r.PaymentMethod,
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

func ToRentalListResponse(rentals []*rentaldomain.Rental) []RentalResponse {
	resp := make([]RentalResponse, 0, len(rentals))
	for _, r := range rentals {
		resp = append(resp, ToRentalResponse(r))
	}
	return resp
}

type CreatePreferenceRequest struct {
	PayerEmail string `json:"payer_email"`
}

type PreferenceResponse struct {
	InitPoint    string `json:"init_point"`
	PreferenceID string `json:"preference_id"`
}

type SendMessageRequest struct {
	Message string `json:"message" binding:"required"`
}

type MessageResponse struct {
	ID        string `json:"id"`
	RentalID  string `json:"rental_id"`
	SenderID  string `json:"sender_id"`
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
}

func ToMessageResponse(m *rentaldomain.Message) MessageResponse {
	return MessageResponse{
		ID:        m.ID.String(),
		RentalID:  m.RentalID.String(),
		SenderID:  m.SenderID.String(),
		Message:   m.Message,
		CreatedAt: m.CreatedAt.Format(time.RFC3339),
	}
}

func ToMessageListResponse(msgs []*rentaldomain.Message) []MessageResponse {
	out := make([]MessageResponse, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, ToMessageResponse(m))
	}
	return out
}
