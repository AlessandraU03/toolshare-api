package tooldomain

import (
	"github.com/google/uuid"
	"time"
)

type Tool struct {
	ID                      uuid.UUID
	OwnerID                 uuid.UUID
	OwnerName               string
	Name                    string
	Description             string
	Category                string
	PhotoURL                string
	EstimatedValue          float64
	DailyRate               float64
	Latitude                float64
	Longitude               float64
	IsAvailable             bool
	ConditionScore          float64
	Brand                   string
	AgeMonths               int
	City                    string
	State                   string
	PriceSource             string
	WantsInsurance          bool
	InsuranceMonthlyPremium float64
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

// InsuranceMonthlyRate es la prima mensual del Respaldo ToolShare: un
// porcentaje del valor estimado de la herramienta.
const InsuranceMonthlyRate = 0.05

// InsuranceClaimRate es lo que cubre el seguro al propietario cuando gana
// una disputa (herramienta dañada) con el seguro activo: 30% del valor
// estimado. Se paga aparte del depósito de garantía capturado al
// solicitante — ver AdminService.ResolveDispute.
const InsuranceClaimRate = 0.30

// CalculateInsuranceClaim calcula lo que cubre el seguro si el propietario
// ganó una disputa; 0 si no tiene el seguro activo.
func (t *Tool) CalculateInsuranceClaim() float64 {
	if !t.WantsInsurance {
		return 0
	}
	return t.EstimatedValue * InsuranceClaimRate
}

// SuggestedDailyRate calcula la tarifa mínima sugerida:
// recuperar el 50% del valor estimado en 30 días de renta.
func (t *Tool) SuggestedDailyRate() float64 {
	if t.EstimatedValue <= 0 {
		return 0
	}
	return t.EstimatedValue * 0.5 / 30
}

// CalculateInsurancePremium calcula la prima mensual del seguro si el
// propietario optó por contratarlo; 0 en caso contrario.
func (t *Tool) CalculateInsurancePremium() float64 {
	if !t.WantsInsurance {
		return 0
	}
	return t.EstimatedValue * InsuranceMonthlyRate
}
