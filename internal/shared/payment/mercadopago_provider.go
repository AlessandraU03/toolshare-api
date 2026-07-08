package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	sharedports "github.com/yourusername/tool-inventory-api/internal/shared/ports"
)

const mpBaseURL = "https://api.mercadopago.com"

// MercadoPagoProvider implementa sharedports.PaymentProvider usando la REST API de MP.
type MercadoPagoProvider struct {
	accessToken string
	httpClient  *http.Client
}

func NewMercadoPagoProvider(accessToken string) sharedports.PaymentProvider {
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

type mpPreferenceRequest struct {
	Items           []mpItem   `json:"items"`
	Payer           *mpPayer   `json:"payer,omitempty"`
	BackURLs        mpBackURLs `json:"back_urls"`
	AutoReturn      string     `json:"auto_return"`
	NotificationURL string     `json:"notification_url,omitempty"`
	ExternalRef     string     `json:"external_reference"`
}

type mpItem struct {
	Title      string  `json:"title"`
	Quantity   int     `json:"quantity"`
	UnitPrice  float64 `json:"unit_price"`
	CurrencyID string  `json:"currency_id"`
}

type mpBackURLs struct {
	Success string `json:"success"`
	Failure string `json:"failure"`
	Pending string `json:"pending"`
}

type mpPreferenceResponse struct {
	ID               string `json:"id"`
	InitPoint        string `json:"init_point"`
	SandboxInitPoint string `json:"sandbox_init_point"`
	Message          string `json:"message,omitempty"`
}

type mpPaymentDetail struct {
	ID          int64  `json:"id"`
	Status      string `json:"status"`
	ExternalRef string `json:"external_reference"`
}

// ── Métodos públicos ──────────────────────────────────────────────────────────

func (p *MercadoPagoProvider) Authorize(ctx context.Context, inp sharedports.AuthorizePaymentInput) (string, error) {
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

func (p *MercadoPagoProvider) CreatePreference(ctx context.Context, inp sharedports.CreatePreferenceInput) (sharedports.CreatePreferenceOutput, error) {
	body := mpPreferenceRequest{
		Items: []mpItem{
			{
				Title:      inp.Title,
				Quantity:   1,
				UnitPrice:  inp.TotalAmount,
				CurrencyID: "MXN",
			},
		},
		BackURLs: mpBackURLs{
			Success: inp.BackURLSuccess,
			Failure: inp.BackURLFailure,
			Pending: inp.BackURLPending,
		},
		AutoReturn:      "approved",
		NotificationURL: inp.NotificationURL,
		ExternalRef:     inp.ExternalRef,
	}
	if inp.PayerEmail != "" {
		body.Payer = &mpPayer{Email: inp.PayerEmail}
	}

	data, err := json.Marshal(body)
	if err != nil {
		return sharedports.CreatePreferenceOutput{}, fmt.Errorf("marshal preference: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mpBaseURL+"/checkout/preferences", bytes.NewReader(data))
	if err != nil {
		return sharedports.CreatePreferenceOutput{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.accessToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return sharedports.CreatePreferenceOutput{}, fmt.Errorf("MP preference request: %w", err)
	}
	defer resp.Body.Close()

	var result mpPreferenceResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return sharedports.CreatePreferenceOutput{}, fmt.Errorf("decode MP preference: %w", err)
	}
	if resp.StatusCode >= 400 {
		return sharedports.CreatePreferenceOutput{}, fmt.Errorf("MP %d: %s", resp.StatusCode, result.Message)
	}

	// Usar sandbox_init_point si el token es de prueba
	initPoint := result.InitPoint
	if strings.HasPrefix(p.accessToken, "TEST-") {
		initPoint = result.SandboxInitPoint
	}

	return sharedports.CreatePreferenceOutput{
		PreferenceID: result.ID,
		InitPoint:    initPoint,
	}, nil
}

func (p *MercadoPagoProvider) GetPaymentInfo(ctx context.Context, paymentID string) (sharedports.PaymentInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mpBaseURL+"/v1/payments/"+paymentID, nil)
	if err != nil {
		return sharedports.PaymentInfo{}, err
	}
	req.Header.Set("Authorization", "Bearer "+p.accessToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return sharedports.PaymentInfo{}, fmt.Errorf("MP get payment: %w", err)
	}
	defer resp.Body.Close()

	var detail mpPaymentDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return sharedports.PaymentInfo{}, fmt.Errorf("decode MP payment: %w", err)
	}
	if resp.StatusCode >= 400 {
		return sharedports.PaymentInfo{}, fmt.Errorf("MP %d: pago no encontrado", resp.StatusCode)
	}

	return sharedports.PaymentInfo{
		ID:          strconv.FormatInt(detail.ID, 10),
		Status:      detail.Status,
		ExternalRef: detail.ExternalRef,
	}, nil
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
