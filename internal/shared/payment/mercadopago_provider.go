package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	sharedports "github.com/yourusername/tool-inventory-api/internal/shared/ports"
)

const mpBaseURL = "https://api.mercadopago.com"

// MercadoPagoProvider implementa sharedports.PaymentProvider usando la REST API de MP.
type MercadoPagoProvider struct {
	accessToken  string
	clientID     string
	clientSecret string
	redirectURI  string
	httpClient   *http.Client
}

func NewMercadoPagoProvider(accessToken, clientID, clientSecret, redirectURI string) sharedports.PaymentProvider {
	return &MercadoPagoProvider{
		accessToken:  accessToken,
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
		httpClient:   &http.Client{Timeout: 15 * time.Second},
	}
}

// ── Estructuras internas de la API de MP ──────────────────────────────────────

type mpPaymentRequest struct {
	TransactionAmount float64           `json:"transaction_amount"`
	Token             string            `json:"token"`
	Description       string            `json:"description"`
	Installments      int               `json:"installments"`
	Capture           bool              `json:"capture"`
	Payer             mpPayer           `json:"payer"`
	Metadata          map[string]string `json:"metadata,omitempty"`
	// ApplicationFee es el monto que retiene la plataforma cuando el pago se
	// crea con el access_token del vendedor (split de marketplace).
	ApplicationFee float64 `json:"application_fee,omitempty"`
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
	Items           []mpItem          `json:"items"`
	Payer           *mpPayer          `json:"payer,omitempty"`
	BackURLs        mpBackURLs        `json:"back_urls"`
	AutoReturn      string            `json:"auto_return"`
	NotificationURL string            `json:"notification_url,omitempty"`
	ExternalRef     string            `json:"external_reference"`
	PaymentMethods  *mpPaymentMethods `json:"payment_methods,omitempty"`
	// MarketplaceFee es el monto que retiene la plataforma cuando la
	// preferencia se crea con el access_token del vendedor.
	MarketplaceFee float64 `json:"marketplace_fee,omitempty"`
}

// mpPaymentMethods restringe qué métodos de pago se muestran en Checkout Pro.
type mpPaymentMethods struct {
	ExcludedPaymentTypes []mpPaymentTypeID `json:"excluded_payment_types,omitempty"`
}

type mpPaymentTypeID struct {
	ID string `json:"id"`
}

// onlyCardPaymentMethods excluye todo lo que no sea tarjeta de crédito/débito.
// "account_money" (saldo de la cuenta de Mercado Pago) faltaba aquí: sin
// excluirlo, Checkout Pro seguía ofreciendo pagar con el saldo de la cuenta
// del comprador sin pedir ninguna tarjeta, aprobando el pago de verdad con
// ese dinero aunque la UI de la app diga "Tarjeta".
func onlyCardPaymentMethods() *mpPaymentMethods {
	excluded := []string{"ticket", "atm", "bank_transfer", "digital_wallet", "digital_currency", "prepaid_card", "account_money"}
	types := make([]mpPaymentTypeID, len(excluded))
	for i, t := range excluded {
		types[i] = mpPaymentTypeID{ID: t}
	}
	return &mpPaymentMethods{ExcludedPaymentTypes: types}
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

type mpCustomerRequest struct {
	Email string `json:"email"`
}

type mpCustomerResponse struct {
	ID      string `json:"id"`
	Message string `json:"message,omitempty"`
}

type mpCardRequest struct {
	Token string `json:"token"`
}

type mpPaymentMethodInfo struct {
	ID string `json:"id"`
}

type mpCardResponse struct {
	ID              string              `json:"id"`
	LastFourDigits  string              `json:"last_four_digits"`
	ExpirationMonth int                 `json:"expiration_month"`
	ExpirationYear  int                 `json:"expiration_year"`
	PaymentMethod   mpPaymentMethodInfo `json:"payment_method"`
	Message         string              `json:"message,omitempty"`
}

// ── Métodos públicos ──────────────────────────────────────────────────────────

func (p *MercadoPagoProvider) Authorize(ctx context.Context, inp sharedports.AuthorizePaymentInput) (string, error) {
	body := mpPaymentRequest{
		TransactionAmount: inp.Amount,
		Token:             inp.CardToken,
		Description:       inp.Description,
		Installments:      1,
		Capture:           false, // pre-autorización: congelar sin cobrar
		Payer:             mpPayer{Email: inp.PayerEmail},
		Metadata:          inp.Metadata,
	}
	if inp.SellerAccessToken != "" {
		body.ApplicationFee = inp.ApplicationFee
	}

	result, err := p.post(ctx, "/v1/payments", body, inp.SellerAccessToken)
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(result.ID, 10), nil
}

func (p *MercadoPagoProvider) Capture(ctx context.Context, paymentID string, amount float64, sellerAccessToken string) error {
	t := true
	body := mpUpdateRequest{
		Capture:           &t,
		TransactionAmount: amount,
	}
	return p.put(ctx, "/v1/payments/"+paymentID, body, sellerAccessToken)
}

func (p *MercadoPagoProvider) Cancel(ctx context.Context, paymentID string, sellerAccessToken string) error {
	return p.put(ctx, "/v1/payments/"+paymentID, mpUpdateRequest{Status: "cancelled"}, sellerAccessToken)
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
		PaymentMethods:  onlyCardPaymentMethods(),
	}
	if inp.PayerEmail != "" {
		body.Payer = &mpPayer{Email: inp.PayerEmail}
	}
	if inp.SellerAccessToken != "" {
		body.MarketplaceFee = inp.ApplicationFee
	}

	data, err := json.Marshal(body)
	if err != nil {
		return sharedports.CreatePreferenceOutput{}, fmt.Errorf("marshal preference: %w", err)
	}

	accessToken := p.tokenOrDefault(inp.SellerAccessToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mpBaseURL+"/checkout/preferences", bytes.NewReader(data))
	if err != nil {
		return sharedports.CreatePreferenceOutput{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

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
	if strings.HasPrefix(accessToken, "TEST-") {
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

// CreateCustomer crea un Customer de Mercado Pago para poder guardarle tarjetas.
func (p *MercadoPagoProvider) CreateCustomer(ctx context.Context, email string) (string, error) {
	data, err := json.Marshal(mpCustomerRequest{Email: email})
	if err != nil {
		return "", fmt.Errorf("marshal customer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mpBaseURL+"/v1/customers", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.accessToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("MP crear customer: %w", err)
	}
	defer resp.Body.Close()

	var result mpCustomerResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode MP customer: %w", err)
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("MP %d: %s", resp.StatusCode, result.Message)
	}
	return result.ID, nil
}

// SaveCard asocia un card_token (tokenizado en el cliente contra la API
// pública de MP) a un Customer existente, dejando la tarjeta guardada para
// futuros cobros sin volver a pedir los datos completos.
func (p *MercadoPagoProvider) SaveCard(ctx context.Context, customerID, cardToken string) (sharedports.SavedCardInfo, error) {
	data, err := json.Marshal(mpCardRequest{Token: cardToken})
	if err != nil {
		return sharedports.SavedCardInfo{}, fmt.Errorf("marshal card: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mpBaseURL+"/v1/customers/"+customerID+"/cards", bytes.NewReader(data))
	if err != nil {
		return sharedports.SavedCardInfo{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.accessToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return sharedports.SavedCardInfo{}, fmt.Errorf("MP guardar tarjeta: %w", err)
	}
	defer resp.Body.Close()

	var result mpCardResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return sharedports.SavedCardInfo{}, fmt.Errorf("decode MP card: %w", err)
	}
	if resp.StatusCode >= 400 {
		return sharedports.SavedCardInfo{}, fmt.Errorf("MP %d: %s", resp.StatusCode, result.Message)
	}

	return sharedports.SavedCardInfo{
		MPCardID:        result.ID,
		CardBrand:       result.PaymentMethod.ID,
		LastFourDigits:  result.LastFourDigits,
		ExpirationMonth: result.ExpirationMonth,
		ExpirationYear:  result.ExpirationYear,
	}, nil
}

// DeleteCard elimina una tarjeta guardada de un Customer.
func (p *MercadoPagoProvider) DeleteCard(ctx context.Context, customerID, mpCardID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, mpBaseURL+"/v1/customers/"+customerID+"/cards/"+mpCardID, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.accessToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("MP eliminar tarjeta: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var result mpCardResponse
		_ = json.NewDecoder(resp.Body).Decode(&result)
		return fmt.Errorf("MP %d: %s", resp.StatusCode, result.Message)
	}
	return nil
}

// ── Helpers HTTP ──────────────────────────────────────────────────────────────

// tokenOrDefault devuelve accessToken si no está vacío, o el access_token de
// la plataforma en caso contrario.
func (p *MercadoPagoProvider) tokenOrDefault(accessToken string) string {
	if accessToken != "" {
		return accessToken
	}
	return p.accessToken
}

func (p *MercadoPagoProvider) post(ctx context.Context, path string, body interface{}, accessToken string) (*mpPaymentResponse, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mpBaseURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.tokenOrDefault(accessToken))
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

func (p *MercadoPagoProvider) put(ctx context.Context, path string, body interface{}, accessToken string) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, mpBaseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.tokenOrDefault(accessToken))

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

// ── OAuth Connect (Marketplace) ────────────────────────────────────────────

const mpOAuthAuthorizeURL = "https://auth.mercadopago.com.mx/authorization"

type mpOAuthTokenRequest struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	GrantType    string `json:"grant_type"`
	Code         string `json:"code,omitempty"`
	RedirectURI  string `json:"redirect_uri,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

type mpOAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	UserID       int64  `json:"user_id"`
	ExpiresIn    int64  `json:"expires_in"`
	Message      string `json:"message,omitempty"`
}

// GetOAuthURL devuelve la URL de autorización de Mercado Pago para vincular
// la cuenta de un propietario (Marketplace/OAuth Connect).
func (p *MercadoPagoProvider) GetOAuthURL(state string) string {
	q := url.Values{}
	q.Set("client_id", p.clientID)
	q.Set("response_type", "code")
	q.Set("platform_id", "mp")
	q.Set("redirect_uri", p.redirectURI)
	q.Set("state", state)
	return mpOAuthAuthorizeURL + "?" + q.Encode()
}

// ExchangeOAuthCode intercambia el "code" del callback de OAuth por los
// tokens de la cuenta del propietario (vendedor).
func (p *MercadoPagoProvider) ExchangeOAuthCode(ctx context.Context, code string) (string, string, string, time.Time, error) {
	result, err := p.oauthTokenRequest(ctx, mpOAuthTokenRequest{
		ClientID:     p.clientID,
		ClientSecret: p.clientSecret,
		GrantType:    "authorization_code",
		Code:         code,
		RedirectURI:  p.redirectURI,
	})
	if err != nil {
		return "", "", "", time.Time{}, err
	}
	expiresAt := time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
	return strconv.FormatInt(result.UserID, 10), result.AccessToken, result.RefreshToken, expiresAt, nil
}

// RefreshSellerToken renueva el access_token de un vendedor usando su refresh_token.
func (p *MercadoPagoProvider) RefreshSellerToken(ctx context.Context, refreshToken string) (string, string, time.Time, error) {
	result, err := p.oauthTokenRequest(ctx, mpOAuthTokenRequest{
		ClientID:     p.clientID,
		ClientSecret: p.clientSecret,
		GrantType:    "refresh_token",
		RefreshToken: refreshToken,
	})
	if err != nil {
		return "", "", time.Time{}, err
	}
	expiresAt := time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
	return result.AccessToken, result.RefreshToken, expiresAt, nil
}

func (p *MercadoPagoProvider) IsMock() bool {
	return false
}

func (p *MercadoPagoProvider) oauthTokenRequest(ctx context.Context, body mpOAuthTokenRequest) (*mpOAuthTokenResponse, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal oauth request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mpBaseURL+"/oauth/token", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("MP oauth request: %w", err)
	}
	defer resp.Body.Close()

	var result mpOAuthTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode MP oauth response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("MP %d: %s", resp.StatusCode, result.Message)
	}
	return &result, nil
}
