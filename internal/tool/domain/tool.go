package tooldomain

import (
	"time"
	"github.com/google/uuid"
)

type Tool struct {
	ID             uuid.UUID
	OwnerID        uuid.UUID
	Name           string
	Description    string
	Category       string
	PhotoURL       string
	EstimatedValue float64
	DailyRate      float64
	Latitude       float64
	Longitude      float64
	IsAvailable    bool
	ConditionScore float64
	Brand          string
	AgeMonths      int
	City           string
	State          string
	PriceSource    string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// SuggestedDailyRate calcula la tarifa mínima sugerida:
// recuperar el 50% del valor estimado en 30 días de renta.
func (t *Tool) SuggestedDailyRate() float64 {
	if t.EstimatedValue <= 0 {
		return 0
	}
	return t.EstimatedValue * 0.5 / 30
}
