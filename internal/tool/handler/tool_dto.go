package toolhandler

import (
	"time"
	tooldomain "github.com/yourusername/tool-inventory-api/internal/tool/domain"
)

type CreateToolRequest struct {
	Name           string  `json:"name"            binding:"required,min=2,max=150" example:"Taladro Bosch 800W"`
	Description    string  `json:"description"                                       example:"Taladro percutor ideal para concreto"`
	Category       string  `json:"category"                                          example:"Construcción"`
	EstimatedValue float64 `json:"estimated_value" binding:"required,gt=0"           example:"2500"`
	DailyRate      float64 `json:"daily_rate"      binding:"required,gt=0"           example:"50"`
	Latitude       float64 `json:"latitude"                                          example:"19.4326"`
	Longitude      float64 `json:"longitude"                                         example:"-99.1332"`
}

type UpdateToolRequest struct {
	Name           *string  `json:"name"            binding:"omitempty,min=2,max=150"`
	Description    *string  `json:"description"`
	Category       *string  `json:"category"`
	EstimatedValue *float64 `json:"estimated_value" binding:"omitempty,gt=0"`
	DailyRate      *float64 `json:"daily_rate"      binding:"omitempty,gt=0"`
	Latitude       *float64 `json:"latitude"`
	Longitude      *float64 `json:"longitude"`
	IsAvailable    *bool    `json:"is_available"`
}

type PricingQueryRequest struct {
	EstimatedValue float64 `form:"estimated_value" binding:"required,gt=0"`
	ScoreCondicion float64 `form:"score_condicion" binding:"required"`
	Category       string  `form:"category"        binding:"required"`
	Brand          string  `form:"brand"           binding:"required"`
}

type ToolResponse struct {
	ID               string  `json:"id"`
	OwnerID          string  `json:"owner_id"`
	Name             string  `json:"name"`
	Description      string  `json:"description"`
	Category         string  `json:"category"`
	PhotoURL         string  `json:"photo_url,omitempty"`
	EstimatedValue   float64 `json:"estimated_value"`
	DailyRate        float64 `json:"daily_rate"`
	SuggestedMinRate float64 `json:"suggested_min_daily_rate"`
	Latitude         float64 `json:"latitude"`
	Longitude        float64 `json:"longitude"`
	IsAvailable      bool    `json:"is_available"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
}

func ToToolResponse(t *tooldomain.Tool) ToolResponse {
	return ToolResponse{
		ID:               t.ID.String(),
		OwnerID:          t.OwnerID.String(),
		Name:             t.Name,
		Description:      t.Description,
		Category:         t.Category,
		PhotoURL:         t.PhotoURL,
		EstimatedValue:   t.EstimatedValue,
		DailyRate:        t.DailyRate,
		SuggestedMinRate: t.SuggestedDailyRate(),
		Latitude:         t.Latitude,
		Longitude:        t.Longitude,
		IsAvailable:      t.IsAvailable,
		CreatedAt:        t.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        t.UpdatedAt.Format(time.RFC3339),
	}
}

func ToToolListResponse(tools []*tooldomain.Tool) []ToolResponse {
	resp := make([]ToolResponse, 0, len(tools))
	for _, t := range tools {
		resp = append(resp, ToToolResponse(t))
	}
	return resp
}

type PredictConditionResponse struct {
	ClasePredicha  string  `json:"clase_predicha"`
	ScoreCondicion float64 `json:"score_condicion"`
}

type AutoValuateResponse struct {
	EstimatedValue float64 `json:"estimated_value"`
	SuggestedDaily float64 `json:"suggested_daily_rate"`
	MinimumDaily   float64 `json:"minimum_daily_rate"`
	Description    string  `json:"description"`
}
