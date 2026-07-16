package reviewservice

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	rentaldomain "github.com/yourusername/tool-inventory-api/internal/rental/domain"
	rentalports "github.com/yourusername/tool-inventory-api/internal/rental/ports"
	reviewdomain "github.com/yourusername/tool-inventory-api/internal/review/domain"
	reviewports "github.com/yourusername/tool-inventory-api/internal/review/ports"
	apperrors "github.com/yourusername/tool-inventory-api/internal/shared/errors"
)

var (
	ErrRentalNotCompleted = errors.New("solo puedes calificar una renta que ya haya finalizado")
	ErrAlreadyReviewed    = errors.New("ya calificaste esta renta")
	ErrInvalidRating      = errors.New("la calificación debe ser entre 1 y 5 estrellas")
)

type reviewService struct {
	reviewRepo reviewports.ReviewRepository
	rentalRepo rentalports.RentalRepository
}

func NewReviewService(reviewRepo reviewports.ReviewRepository, rentalRepo rentalports.RentalRepository) reviewports.ReviewService {
	return &reviewService{reviewRepo: reviewRepo, rentalRepo: rentalRepo}
}

func (s *reviewService) Create(ctx context.Context, inp reviewports.CreateReviewInput) (*reviewdomain.Review, error) {
	if inp.Rating < 1 || inp.Rating > 5 {
		return nil, ErrInvalidRating
	}

	rental, err := s.rentalRepo.FindByID(ctx, inp.RentalID)
	if err != nil {
		return nil, err
	}

	if rental.Status != rentaldomain.RentalStatusCompleted {
		return nil, ErrRentalNotCompleted
	}

	var targetType reviewdomain.TargetType
	var targetID uuid.UUID

	switch inp.AuthorID {
	case rental.OwnerID:
		// El propietario califica al solicitante (reputación como arrendatario).
		targetType = reviewdomain.TargetTypeUser
		targetID = rental.RequesterID
	case rental.RequesterID:
		// El solicitante califica cómo estuvo la herramienta/renta.
		targetType = reviewdomain.TargetTypeTool
		targetID = rental.ToolID
	default:
		return nil, apperrors.ErrForbidden
	}

	exists, err := s.reviewRepo.ExistsByRentalAndAuthor(ctx, inp.RentalID, inp.AuthorID)
	if err != nil {
		return nil, fmt.Errorf("verificar reseña previa: %w", err)
	}
	if exists {
		return nil, ErrAlreadyReviewed
	}

	review := &reviewdomain.Review{
		RentalID:   inp.RentalID,
		AuthorID:   inp.AuthorID,
		TargetType: targetType,
		TargetID:   targetID,
		Rating:     inp.Rating,
		Comment:    inp.Comment,
	}

	return s.reviewRepo.Create(ctx, review)
}

func (s *reviewService) ListForTool(ctx context.Context, toolID uuid.UUID) ([]*reviewdomain.Review, reviewports.RatingSummary, error) {
	reviews, err := s.reviewRepo.FindByTarget(ctx, reviewdomain.TargetTypeTool, toolID)
	if err != nil {
		return nil, reviewports.RatingSummary{}, err
	}
	summary, err := s.reviewRepo.GetSummary(ctx, reviewdomain.TargetTypeTool, toolID)
	if err != nil {
		return nil, reviewports.RatingSummary{}, err
	}
	return reviews, summary, nil
}

func (s *reviewService) ListForUser(ctx context.Context, userID uuid.UUID) ([]*reviewdomain.Review, reviewports.RatingSummary, error) {
	reviews, err := s.reviewRepo.FindByTarget(ctx, reviewdomain.TargetTypeUser, userID)
	if err != nil {
		return nil, reviewports.RatingSummary{}, err
	}
	summary, err := s.reviewRepo.GetSummary(ctx, reviewdomain.TargetTypeUser, userID)
	if err != nil {
		return nil, reviewports.RatingSummary{}, err
	}
	return reviews, summary, nil
}
