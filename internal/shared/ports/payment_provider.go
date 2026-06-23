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

// PaymentProvider es el puerto de salida para la pasarela de pagos.
// Implementado por MercadoPagoProvider (producción) o MockPaymentProvider (desarrollo).
type PaymentProvider interface {
	// Authorize congela los fondos sin cobrar (capture: false en MP).
	// Devuelve el ID del pago en la pasarela.
	Authorize(ctx context.Context, inp AuthorizePaymentInput) (paymentID string, err error)

	// Capture cobra el monto indicado. Para captura parcial (liberar depósito)
	// se pasa solo el monto de la renta, no el total autorizado.
	Capture(ctx context.Context, paymentID string, amount float64) error

	// Cancel libera la pre-autorización y devuelve los fondos al solicitante.
	Cancel(ctx context.Context, paymentID string) error
}
