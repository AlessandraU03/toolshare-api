package toolports

import (
	"context"
	"github.com/google/uuid"
	sharedports "github.com/yourusername/tool-inventory-api/internal/shared/ports"
	tooldomain "github.com/yourusername/tool-inventory-api/internal/tool/domain"
	"io"
)

type CreateToolInput struct {
	OwnerID        uuid.UUID
	Name           string
	Description    string
	Category       string
	EstimatedValue float64
	DailyRate      float64
	Latitude       float64
	Longitude      float64
	Brand          string
	AgeMonths      int
	City           string
	State          string
	ConditionScore float64
}

type UpdateToolInput struct {
	Name           *string
	Description    *string
	Category       *string
	EstimatedValue *float64
	DailyRate      *float64
	Latitude       *float64
	Longitude      *float64
	IsAvailable    *bool
}

type UploadPhotoInput struct {
	ToolID      uuid.UUID
	OwnerID     uuid.UUID
	Filename    string
	Content     io.Reader
	ContentType string
}

type PricingSuggestion struct {
	EstimatedValue      float64 `json:"estimated_value"`
	SuggestedDaily      float64 `json:"suggested_daily_rate"`
	MinimumDaily        float64 `json:"minimum_daily_rate"`
	GuaranteeCap        float64 `json:"guarantee_cap"`
	SuggestedDeductible float64 `json:"suggested_deductible"`
	Description         string  `json:"description"`
}

type PredictConditionOutput struct {
	ClasePredicha  string  `json:"clase_predicha"`
	ScoreCondicion float64 `json:"score_condicion"`
}

type AutoValuateOutput struct {
	EstimatedValue       float64 `json:"estimated_value"`
	SuggestedDaily       float64 `json:"suggested_daily_rate"`
	MinimumDaily         float64 `json:"minimum_daily_rate"`
	Description          string  `json:"description"`
	RequiresManualReview bool    `json:"requires_manual_review"`
}

type ExtractTicketPriceOutput struct {
	Valid         bool    `json:"valid"`
	DetectedPrice float64 `json:"detected_price"`
	Confidence    string  `json:"confidence"`
	Error         string  `json:"error,omitempty"`
}

type ToolService interface {
	Create(ctx context.Context, inp CreateToolInput) (*tooldomain.Tool, error)
	GetByID(ctx context.Context, id uuid.UUID) (*tooldomain.Tool, error)
	List(ctx context.Context, filter ToolFilter) ([]*tooldomain.Tool, error)
	Update(ctx context.Context, id uuid.UUID, ownerID uuid.UUID, inp UpdateToolInput) (*tooldomain.Tool, error)
	Delete(ctx context.Context, id uuid.UUID, ownerID uuid.UUID) error
	UploadPhoto(ctx context.Context, inp UploadPhotoInput) (*tooldomain.Tool, error)
	GetPricingSuggestion(ctx context.Context, estimatedValue float64, scoreCondicion float64, category string, brand string, name string, ageMonths int) *PricingSuggestion
	PredictCondition(ctx context.Context, filename string, content io.Reader, contentType string) (*PredictConditionOutput, error)
	AutoValuate(ctx context.Context, name string, scoreCondicion float64, category string, brand string, ageMonths int, precioBaseManual *float64, ticketValidado bool) (*AutoValuateOutput, error)
	ExtractTicketPrice(ctx context.Context, filename string, content io.Reader, contentType string) (*ExtractTicketPriceOutput, error)

	// CreateInsurancePreference crea una preferencia de Checkout Pro en MP para
	// que el propietario pague la prima mensual del seguro de una herramienta.
	CreateInsurancePreference(ctx context.Context, toolID uuid.UUID, ownerID uuid.UUID) (sharedports.CreatePreferenceOutput, error)
	// ConfirmInsurancePayment verifica el pago directamente con MP y activa el seguro.
	ConfirmInsurancePayment(ctx context.Context, toolID uuid.UUID, ownerID uuid.UUID, paymentID string) (*tooldomain.Tool, error)
	// CancelInsurance desactiva el seguro; no genera reembolso de la prima ya pagada.
	CancelInsurance(ctx context.Context, toolID uuid.UUID, ownerID uuid.UUID) (*tooldomain.Tool, error)
}
