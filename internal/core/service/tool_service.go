package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/tool-inventory-api/internal/core/domain"
	"github.com/yourusername/tool-inventory-api/internal/core/ports/input"
	"github.com/yourusername/tool-inventory-api/internal/core/ports/output"
)

type toolService struct {
	toolRepo    output.ToolRepository
	fileStorage output.FileStorage
}

func NewToolService(toolRepo output.ToolRepository, fileStorage output.FileStorage) input.ToolService {
	return &toolService{toolRepo: toolRepo, fileStorage: fileStorage}
}

func (s *toolService) Create(ctx context.Context, inp input.CreateToolInput) (*domain.Tool, error) {
	tool := &domain.Tool{
		OwnerID:        inp.OwnerID,
		Name:           inp.Name,
		Description:    inp.Description,
		Category:       inp.Category,
		EstimatedValue: inp.EstimatedValue,
		DailyRate:      inp.DailyRate,
		IsAvailable:    true,
	}

	// Aplicar mínimo del 50% del valor en 30 días
	if minRate := tool.SuggestedDailyRate(); tool.DailyRate < minRate {
		tool.DailyRate = minRate
	}

	return s.toolRepo.Create(ctx, tool)
}

func (s *toolService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Tool, error) {
	return s.toolRepo.FindByID(ctx, id)
}

func (s *toolService) List(ctx context.Context, filter output.ToolFilter) ([]*domain.Tool, error) {
	return s.toolRepo.FindAll(ctx, filter)
}

func (s *toolService) Update(ctx context.Context, id uuid.UUID, ownerID uuid.UUID, inp input.UpdateToolInput) (*domain.Tool, error) {
	tool, err := s.toolRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if tool.OwnerID != ownerID {
		return nil, output.ErrForbidden
	}

	if inp.Name != nil {
		tool.Name = *inp.Name
	}
	if inp.Description != nil {
		tool.Description = *inp.Description
	}
	if inp.Category != nil {
		tool.Category = *inp.Category
	}
	if inp.EstimatedValue != nil {
		tool.EstimatedValue = *inp.EstimatedValue
	}
	if inp.DailyRate != nil {
		tool.DailyRate = *inp.DailyRate
		if minRate := tool.SuggestedDailyRate(); tool.DailyRate < minRate {
			tool.DailyRate = minRate
		}
	}
	if inp.IsAvailable != nil {
		tool.IsAvailable = *inp.IsAvailable
	}

	return s.toolRepo.Update(ctx, tool)
}

func (s *toolService) Delete(ctx context.Context, id uuid.UUID, ownerID uuid.UUID) error {
	tool, err := s.toolRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if tool.OwnerID != ownerID {
		return output.ErrForbidden
	}
	return s.toolRepo.Delete(ctx, id)
}

func (s *toolService) UploadPhoto(ctx context.Context, inp input.UploadPhotoInput) (*domain.Tool, error) {
	tool, err := s.toolRepo.FindByID(ctx, inp.ToolID)
	if err != nil {
		return nil, err
	}
	if tool.OwnerID != inp.OwnerID {
		return nil, output.ErrForbidden
	}

	filename := fmt.Sprintf("tools/%s_%d_%s", inp.ToolID.String(), time.Now().Unix(), inp.Filename)
	url, err := s.fileStorage.Upload(ctx, filename, inp.Content, inp.ContentType)
	if err != nil {
		return nil, err
	}

	tool.PhotoURL = url
	return s.toolRepo.Update(ctx, tool)
}

func (s *toolService) GetPricingSuggestion(estimatedValue float64) *input.PricingSuggestion {
	minDaily := estimatedValue * 0.5 / 30
	return &input.PricingSuggestion{
		EstimatedValue: estimatedValue,
		SuggestedDaily: minDaily,
		MinimumDaily:   minDaily,
		Description:    "Precio mínimo sugerido: recuperar el 50% del valor en 30 días de renta",
	}
}
