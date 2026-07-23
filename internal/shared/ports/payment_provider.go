package sharedports

import (
	"context"
	"time"
)

// AuthorizePaymentInput contiene los datos necesarios para pre-autorizar un pago.
type AuthorizePaymentInput struct {
	Amount      float64 // deducible + monto total de la renta
	CardToken   string  // token generado por el SDK de Mercado Pago en el frontend
	Description string
	PayerEmail  string
	Metadata    map[string]string // ej. rental_id

	// SellerAccessToken, si viene, hace que el pago se cree usando la cuenta
	// de Mercado Pago del propietario (vendedor) en vez de la de la
	// plataforma — es lo que permite el split automático de fondos. Si va
	// vacío, se usa el access_token de la plataforma como antes (ej. para
	// flujos que no reparten dinero, como la suscripción Pro).
	SellerAccessToken string
	// ApplicationFee es el monto (no porcentaje) que se queda la plataforma;
	// el resto se deposita en la cuenta del SellerAccessToken. Solo aplica
	// si SellerAccessToken no está vacío.
	ApplicationFee float64
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

	// SellerAccessToken / ApplicationFee: mismo propósito que en
	// AuthorizePaymentInput, pero para el flujo de Checkout Pro
	// (marketplace_fee).
	SellerAccessToken string
	ApplicationFee    float64
}

// CreatePreferenceOutput contiene el resultado de crear una preferencia.
type CreatePreferenceOutput struct {
	PreferenceID string
	InitPoint    string // URL que el frontend carga en el WebView
}

// PaymentInfo representa la información de un pago obtenida desde MP.
type PaymentInfo struct {
	ID            string
	Status        string // approved, pending, rejected, cancelled…
	ExternalRef   string // rental ID guardado como external_reference
	PaymentTypeID string // credit_card, debit_card, account_money, ticket…
}

// SavedCardInfo representa una tarjeta guardada en el Customer de Mercado Pago.
type SavedCardInfo struct {
	MPCardID        string
	CardBrand       string // "visa", "master", etc.
	LastFourDigits  string
	ExpirationMonth int
	ExpirationYear  int
}

// PaymentProvider es el puerto de salida para la pasarela de pagos.
// Implementado por MercadoPagoProvider (producción) o MockPaymentProvider (desarrollo).
type PaymentProvider interface {
	// Authorize congela los fondos sin cobrar (capture: false en MP).
	Authorize(ctx context.Context, inp AuthorizePaymentInput) (paymentID string, err error)

	// Capture cobra el monto indicado. sellerAccessToken debe ser el mismo
	// usado al crear el pago en Authorize (vacío si no hubo split).
	Capture(ctx context.Context, paymentID string, amount float64, sellerAccessToken string) error

	// Cancel libera la pre-autorización y devuelve los fondos al solicitante.
	// sellerAccessToken debe ser el mismo usado al crear el pago en Authorize.
	Cancel(ctx context.Context, paymentID string, sellerAccessToken string) error

	// Refund devuelve el dinero de un pago ya aprobado/capturado. amount=0
	// hace un reembolso total; amount>0, uno parcial. sellerAccessToken debe
	// ser el mismo usado al crear el pago.
	Refund(ctx context.Context, paymentID string, amount float64, sellerAccessToken string) error

	// CreatePreference crea una preferencia de Checkout Pro y devuelve el init_point.
	CreatePreference(ctx context.Context, inp CreatePreferenceInput) (CreatePreferenceOutput, error)

	// GetPaymentInfo consulta el estado de un pago por su ID.
	GetPaymentInfo(ctx context.Context, paymentID string) (PaymentInfo, error)

	// CreateCustomer crea un Customer de Mercado Pago para guardar tarjetas a su nombre.
	CreateCustomer(ctx context.Context, email string) (customerID string, err error)

	// SaveCard asocia un card_token (generado en el cliente) a un Customer existente.
	SaveCard(ctx context.Context, customerID, cardToken string) (SavedCardInfo, error)

	// DeleteCard elimina una tarjeta guardada de un Customer.
	DeleteCard(ctx context.Context, customerID, mpCardID string) error

	// ── Marketplace / OAuth Connect ────────────────────────────────────────
	// Permite que un propietario vincule su propia cuenta de Mercado Pago
	// para recibir directamente su parte de cada pago (split automático).

	// GetOAuthURL devuelve la URL de autorización de Mercado Pago a la que
	// debe ser redirigido el propietario para vincular su cuenta.
	GetOAuthURL(state string) string

	// ExchangeOAuthCode intercambia el "code" recibido en el callback de
	// OAuth por los tokens de la cuenta del propietario (vendedor).
	ExchangeOAuthCode(ctx context.Context, code string) (sellerUserID, accessToken, refreshToken string, expiresAt time.Time, err error)

	// RefreshSellerToken renueva el access_token de un vendedor usando su
	// refresh_token, cuando el guardado ya expiró o está por expirar.
	RefreshSellerToken(ctx context.Context, refreshToken string) (accessToken, newRefreshToken string, expiresAt time.Time, err error)

	// IsMock indica si el provider es el simulado de desarrollo (true) o uno
	// real que mueve dinero (false). Se usa para no exigir que el propietario
	// haya vinculado su cuenta de MP cuando no hay credenciales reales.
	IsMock() bool
}
