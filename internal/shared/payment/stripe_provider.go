package payment

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	sharedports "github.com/yourusername/tool-inventory-api/internal/shared/ports"
)

const stripeBaseURL = "https://api.stripe.com"

// StripeProvider implementa sharedports.PaymentProvider usando la REST API de Stripe.
type StripeProvider struct {
	secretKey  string
	httpClient *http.Client
}

func NewStripeProvider(secretKey string) sharedports.PaymentProvider {
	return &StripeProvider{
		secretKey:  secretKey,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (p *StripeProvider) Authorize(ctx context.Context, inp sharedports.AuthorizePaymentInput) (string, error) {
	// Stripe maneja montos en centavos (ej: $100.00 MXN = 10000 centavos)
	amountInCents := int64(inp.Amount * 100)

	form := url.Values{}
	form.Set("amount", strconv.FormatInt(amountInCents, 10))
	form.Set("currency", "mxn")
	form.Set("payment_method_types[0]", "card")
	form.Set("capture_method", "manual") // pre-autorización (hold)
	if strings.HasPrefix(inp.CardToken, "tok_") {
		form.Set("payment_method_data[type]", "card")
		form.Set("payment_method_data[card][token]", inp.CardToken)
	} else {
		form.Set("payment_method", inp.CardToken)
	}
	form.Set("confirm", "true")
	form.Set("description", inp.Description)
	if inp.PayerEmail != "" {
		form.Set("receipt_email", inp.PayerEmail)
	}
	for k, v := range inp.Metadata {
		form.Set("metadata["+k+"]", v)
	}

	result, err := p.post(ctx, "/v1/payment_intents", form)
	if err != nil {
		return "", err
	}
	return result.ID, nil
}

func (p *StripeProvider) Capture(ctx context.Context, paymentID string, amount float64) error {
	amountInCents := int64(amount * 100)
	form := url.Values{}
	form.Set("amount_to_capture", strconv.FormatInt(amountInCents, 10))

	_, err := p.post(ctx, fmt.Sprintf("/v1/payment_intents/%s/capture", paymentID), form)
	return err
}

func (p *StripeProvider) Cancel(ctx context.Context, paymentID string) error {
	_, err := p.post(ctx, fmt.Sprintf("/v1/payment_intents/%s/cancel", paymentID), url.Values{})
	return err
}

// CreatePreference no está implementado para Stripe: el flujo de Checkout Pro
// (preferencia + redirect) es específico de Mercado Pago. Stripe se usa aquí
// solo con el flujo directo de tarjeta (Authorize/Capture/Cancel).
func (p *StripeProvider) CreatePreference(ctx context.Context, inp sharedports.CreatePreferenceInput) (sharedports.CreatePreferenceOutput, error) {
	return sharedports.CreatePreferenceOutput{}, fmt.Errorf("CreatePreference no está implementado para Stripe")
}

// GetPaymentInfo no está implementado para Stripe: no hay webhook de Stripe integrado.
func (p *StripeProvider) GetPaymentInfo(ctx context.Context, paymentID string) (sharedports.PaymentInfo, error) {
	return sharedports.PaymentInfo{}, fmt.Errorf("GetPaymentInfo no está implementado para Stripe")
}

type stripePaymentResponse struct {
	ID    string `json:"id"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (p *StripeProvider) post(ctx context.Context, path string, form url.Values) (*stripePaymentResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, stripeBaseURL+path, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+p.secretKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("petición a Stripe fallida: %w", err)
	}
	defer resp.Body.Close()

	var result stripePaymentResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decodificar respuesta de Stripe: %w", err)
	}

	if resp.StatusCode >= 400 {
		msg := "error desconocido de Stripe"
		if result.Error != nil {
			msg = result.Error.Message
		}
		return nil, fmt.Errorf("Stripe API (código %d): %s", resp.StatusCode, msg)
	}

	return &result, nil
}
