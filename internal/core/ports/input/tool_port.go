package input

import (
	"context"
	"io"

	"github.com/google/uuid"
	"github.com/yourusername/tool-inventory-api/internal/core/domain"
	"github.com/yourusername/tool-inventory-api/internal/core/ports/output"
)

type CreateToolInput struct {
	OwnerID        uuid.UUID
	Name           string
	Description    string
	Category       string
	EstimatedValue float64
	DailyRate      float64
}

type UpdateToolInput struct {
	Name           *string
	Description    *string
	Category       *string
	EstimatedValue *float64
	DailyRate      *float64
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
	EstimatedValue float64 `json:"estimated_value"`
	SuggestedDaily float64 `json:"suggested_daily_rate"`
	MinimumDaily   float64 `json:"minimum_daily_rate"`
	Description    string  `json:"description"`
}

type ToolService interface {
	Create(ctx context.Context, inp CreateToolInput) (*domain.Tool, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Tool, error)
	List(ctx context.Context, filter output.ToolFilter) ([]*domain.Tool, error)
	Update(ctx context.Context, id uuid.UUID, ownerID uuid.UUID, inp UpdateToolInput) (*domain.Tool, error)
	Delete(ctx context.Context, id uuid.UUID, ownerID uuid.UUID) error
	UploadPhoto(ctx context.Context, inp UploadPhotoInput) (*domain.Tool, error)
	GetPricingSuggestion(estimatedValue float64) *PricingSuggestion
}
