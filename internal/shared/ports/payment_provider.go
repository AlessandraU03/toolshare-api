package sharedports

import (
	"context"
)

// AuthorizePaymentInput contiene los datos necesarios para pre-autorizar un pago.
type AuthorizePaymentInput struct {
	Amount      float64           // deducible + monto total de la renta
	CardToken   string            // token generado por el SDK de Mercado Pago en el frontend
	Description string
	PayerEmail  string
	Metadata    map[string]string // ej. rental_id
}

// CreatePreferenceInput contiene los datos para crear una preferencia de Checkout Pro.
type CreatePreferenceInput struct {
	Title           string
	TotalAmount     float64
	PayerEmail      string
	ExternalRef     string // rental ID — usado para identificar la renta en el webhook
	NotificationURL string // URL pública donde MP enviará el webhook
	BackURLSuccess  string
	BackURLFailure  string
	BackURLPending  string
}

// CreatePreferenceOutput contiene el resultado de crear una preferencia.
type CreatePreferenceOutput struct {
	PreferenceID string
	InitPoint    string // URL que el frontend carga en el WebView
}

// PaymentInfo representa la información de un pago obtenida desde MP.
type PaymentInfo struct {
	ID          string
	Status      string // approved, pending, rejected, cancelled…
	ExternalRef string // rental ID guardado como external_reference
}

// PaymentProvider es el puerto de salida para la pasarela de pagos.
// Implementado por MercadoPagoProvider (producción) o MockPaymentProvider (desarrollo).
type PaymentProvider interface {
	// Authorize congela los fondos sin cobrar (capture: false en MP).
	Authorize(ctx context.Context, inp AuthorizePaymentInput) (paymentID string, err error)

	// Capture cobra el monto indicado.
	Capture(ctx context.Context, paymentID string, amount float64) error

	// Cancel libera la pre-autorización y devuelve los fondos al solicitante.
	Cancel(ctx context.Context, paymentID string) error

	// CreatePreference crea una preferencia de Checkout Pro y devuelve el init_point.
	CreatePreference(ctx context.Context, inp CreatePreferenceInput) (CreatePreferenceOutput, error)

	// GetPaymentInfo consulta el estado de un pago por su ID.
	GetPaymentInfo(ctx context.Context, paymentID string) (PaymentInfo, error)
}
