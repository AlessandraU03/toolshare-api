package payment

import (
	"context"
	"fmt"
	"time"

	"github.com/yourusername/tool-inventory-api/internal/core/ports/output"
)

// MockPaymentProvider simula Mercado Pago en desarrollo/testing.
// Se usa automáticamente cuando MP_ACCESS_TOKEN no está configurado.
type MockPaymentProvider struct{}

func NewMockPaymentProvider() output.PaymentProvider {
	return &MockPaymentProvider{}
}

func (p *MockPaymentProvider) Authorize(_ context.Context, inp output.AuthorizePaymentInput) (string, error) {
	return fmt.Sprintf("MOCK_%d", time.Now().UnixMilli()), nil
}

func (p *MockPaymentProvider) Capture(_ context.Context, paymentID string, amount float64) error {
	return nil
}

func (p *MockPaymentProvider) Cancel(_ context.Context, paymentID string) error {
	return nil
}
