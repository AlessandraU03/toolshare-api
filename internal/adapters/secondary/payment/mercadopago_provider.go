package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/tool-inventory-api/internal/core/ports/output"
)

const mpBaseURL = "https://api.mercadopago.com"

// MercadoPagoProvider implementa output.PaymentProvider usando la REST API de MP.
type MercadoPagoProvider struct {
	accessToken string
	httpClient  *http.Client
}

func NewMercadoPagoProvider(accessToken string) output.PaymentProvider {
	return &MercadoPagoProvider{
		accessToken: accessToken,
		httpClient:  &http.Client{Timeout: 15 * time.Second},
	}
}

// ── Estructuras internas de la API de MP ──────────────────────────────────────

type mpPaymentRequest struct {
	TransactionAmount float64           `json:"transaction_amount"`
	Token            string            `json:"token"`
	Description      string            `json:"description"`
	Installments     int               `json:"installments"`
	Capture          bool              `json:"capture"`
	Payer            mpPayer           `json:"payer"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

type mpPayer struct {
	Email string `json:"email"`
}

type mpPaymentResponse struct {
	ID      int64  `json:"id"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type mpUpdateRequest struct {
	Status            string  `json:"status,omitempty"`
	Capture           *bool   `json:"capture,omitempty"`
	TransactionAmount float64 `json:"transaction_amount,omitempty"`
}

// ── Métodos públicos ──────────────────────────────────────────────────────────

func (p *MercadoPagoProvider) Authorize(ctx context.Context, inp output.AuthorizePaymentInput) (string, error) {
	body := mpPaymentRequest{
		TransactionAmount: inp.Amount,
		Token:            inp.CardToken,
		Description:      inp.Description,
		Installments:     1,
		Capture:          false, // pre-autorización: congelar sin cobrar
		Payer:            mpPayer{Email: inp.PayerEmail},
		Metadata:         inp.Metadata,
	}

	result, err := p.post(ctx, "/v1/payments", body)
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(result.ID, 10), nil
}

func (p *MercadoPagoProvider) Capture(ctx context.Context, paymentID string, amount float64) error {
	t := true
	body := mpUpdateRequest{
		Capture:           &t,
		TransactionAmount: amount,
	}
	return p.put(ctx, "/v1/payments/"+paymentID, body)
}

func (p *MercadoPagoProvider) Cancel(ctx context.Context, paymentID string) error {
	return p.put(ctx, "/v1/payments/"+paymentID, mpUpdateRequest{Status: "cancelled"})
}

// ── Helpers HTTP ──────────────────────────────────────────────────────────────

func (p *MercadoPagoProvider) post(ctx context.Context, path string, body interface{}) (*mpPaymentResponse, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mpBaseURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.accessToken)
	req.Header.Set("X-Idempotency-Key", uuid.New().String())

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("MP request: %w", err)
	}
	defer resp.Body.Close()

	var result mpPaymentResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode MP response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("MP %d: %s", resp.StatusCode, result.Message)
	}
	return &result, nil
}

func (p *MercadoPagoProvider) put(ctx context.Context, path string, body interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, mpBaseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.accessToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("MP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var result mpPaymentResponse
		_ = json.NewDecoder(resp.Body).Decode(&result)
		return fmt.Errorf("MP %d: %s", resp.StatusCode, result.Message)
	}
	return nil
}
