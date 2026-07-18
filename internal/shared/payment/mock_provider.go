package payment

import (
	"context"
	"fmt"
	"time"

	sharedports "github.com/yourusername/tool-inventory-api/internal/shared/ports"
)

// MockPaymentProvider simula Mercado Pago en desarrollo/testing.
// Se usa automáticamente cuando MP_ACCESS_TOKEN no está configurado.
type MockPaymentProvider struct{}

func NewMockPaymentProvider() sharedports.PaymentProvider {
	return &MockPaymentProvider{}
}

func (p *MockPaymentProvider) Authorize(_ context.Context, inp sharedports.AuthorizePaymentInput) (string, error) {
	return fmt.Sprintf("MOCK_%d", time.Now().UnixMilli()), nil
}

func (p *MockPaymentProvider) Capture(_ context.Context, paymentID string, amount float64) error {
	return nil
}

func (p *MockPaymentProvider) Cancel(_ context.Context, paymentID string) error {
	return nil
}

func (p *MockPaymentProvider) CreatePreference(_ context.Context, inp sharedports.CreatePreferenceInput) (sharedports.CreatePreferenceOutput, error) {
	return sharedports.CreatePreferenceOutput{
		PreferenceID: fmt.Sprintf("MOCK_PREF_%d", time.Now().UnixMilli()),
		InitPoint:    "https://www.mercadopago.com.mx/checkout/v1/redirect?pref_id=MOCK_TEST",
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
