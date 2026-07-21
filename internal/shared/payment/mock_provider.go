package payment

import (
	"context"
	"fmt"
	"net/url"
	"time"

	sharedports "github.com/yourusername/tool-inventory-api/internal/shared/ports"
)

// MockPaymentProvider simula Mercado Pago en desarrollo/testing.
// Se usa automáticamente cuando MP_ACCESS_TOKEN no está configurado.
type MockPaymentProvider struct {
	// redirectURI es el callback propio del backend (MP_OAUTH_REDIRECT_URI).
	// GetOAuthURL "salta" directo ahí con un code falso, en vez de abrir el
	// dominio real de Mercado Pago (que no tiene esa ruta y daría 404),
	// simulando una autorización instantánea para pruebas locales.
	redirectURI string
	// baseURL (esquema+host+puerto, sin path) derivado de redirectURI. Se usa
	// para armar la URL del "checkout" simulado en CreatePreference, que en
	// vez de abrir mercadopago.com.mx (inexistente para una preferencia
	// falsa) apunta a un endpoint propio del backend que aprueba el pago al
	// instante y redirige de vuelta a la app.
	baseURL string
}

func NewMockPaymentProvider(redirectURI string) sharedports.PaymentProvider {
	baseURL := redirectURI
	if u, err := url.Parse(redirectURI); err == nil && u.Scheme != "" && u.Host != "" {
		baseURL = u.Scheme + "://" + u.Host
	}
	return &MockPaymentProvider{redirectURI: redirectURI, baseURL: baseURL}
}

func (p *MockPaymentProvider) Authorize(_ context.Context, inp sharedports.AuthorizePaymentInput) (string, error) {
	return fmt.Sprintf("MOCK_%d", time.Now().UnixMilli()), nil
}

func (p *MockPaymentProvider) Capture(_ context.Context, paymentID string, amount float64, sellerAccessToken string) error {
	return nil
}

func (p *MockPaymentProvider) Cancel(_ context.Context, paymentID string, sellerAccessToken string) error {
	return nil
}

func (p *MockPaymentProvider) CreatePreference(_ context.Context, inp sharedports.CreatePreferenceInput) (sharedports.CreatePreferenceOutput, error) {
	q := url.Values{}
	q.Set("external_reference", inp.ExternalRef)
	q.Set("back_url", inp.BackURLSuccess)
	return sharedports.CreatePreferenceOutput{
		PreferenceID: fmt.Sprintf("MOCK_PREF_%d", time.Now().UnixMilli()),
		InitPoint:    p.baseURL + "/api/mock/mp-checkout?" + q.Encode(),
	}, nil
}

func (p *MockPaymentProvider) GetPaymentInfo(_ context.Context, paymentID string) (sharedports.PaymentInfo, error) {
	return sharedports.PaymentInfo{
		ID:          paymentID,
		Status:      "approved",
		ExternalRef: "",
	}, nil
}

func (p *MockPaymentProvider) CreateCustomer(_ context.Context, _ string) (string, error) {
	return fmt.Sprintf("MOCK_CUSTOMER_%d", time.Now().UnixMilli()), nil
}

func (p *MockPaymentProvider) SaveCard(_ context.Context, _ string, cardToken string) (sharedports.SavedCardInfo, error) {
	return sharedports.SavedCardInfo{
		MPCardID:        fmt.Sprintf("MOCK_CARD_%d", time.Now().UnixMilli()),
		CardBrand:       "visa",
		LastFourDigits:  "1111",
		ExpirationMonth: 12,
		ExpirationYear:  time.Now().Year() + 2,
	}, nil
}

func (p *MockPaymentProvider) DeleteCard(_ context.Context, _ string, _ string) error {
	return nil
}

func (p *MockPaymentProvider) GetOAuthURL(state string) string {
	q := url.Values{}
	q.Set("code", "MOCK_CODE")
	q.Set("state", state)
	return p.redirectURI + "?" + q.Encode()
}

func (p *MockPaymentProvider) ExchangeOAuthCode(_ context.Context, code string) (string, string, string, time.Time, error) {
	return fmt.Sprintf("MOCK_SELLER_%d", time.Now().UnixMilli()),
		fmt.Sprintf("MOCK_SELLER_TOKEN_%d", time.Now().UnixMilli()),
		fmt.Sprintf("MOCK_SELLER_REFRESH_%d", time.Now().UnixMilli()),
		time.Now().Add(180 * 24 * time.Hour),
		nil
}

func (p *MockPaymentProvider) RefreshSellerToken(_ context.Context, refreshToken string) (string, string, time.Time, error) {
	return fmt.Sprintf("MOCK_SELLER_TOKEN_%d", time.Now().UnixMilli()),
		refreshToken,
		time.Now().Add(180 * 24 * time.Hour),
		nil
}
