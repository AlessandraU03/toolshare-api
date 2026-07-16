package reviewports

import (
	"context"

	"github.com/google/uuid"
	reviewdomain "github.com/yourusername/tool-inventory-api/internal/review/domain"
)

type CreateReviewInput struct {
	RentalID uuid.UUID
	AuthorID uuid.UUID
	Rating   int
	Comment  string
}

type RatingSummary struct {
	Average float64
	Count   int
}

type ReviewRepository interface {
	Create(ctx context.Context, review *reviewdomain.Review) (*reviewdomain.Review, error)
	ExistsByRentalAndAuthor(ctx context.Context, rentalID, authorID uuid.UUID) (bool, error)
	FindByTarget(ctx context.Context, targetType reviewdomain.TargetType, targetID uuid.UUID) ([]*reviewdomain.Review, error)
	GetSummary(ctx context.Context, targetType reviewdomain.TargetType, targetID uuid.UUID) (RatingSummary, error)
}

type ReviewService interface {
	// Create infiere la dirección de la reseña a partir de la renta:
	// si el autor es el propietario, califica al solicitante (TargetTypeUser);
	// si el autor es el solicitante, califica la herramienta (TargetTypeTool).
	Create(ctx context.Context, inp CreateReviewInput) (*reviewdomain.Review, error)
	ListForTool(ctx context.Context, toolID uuid.UUID) ([]*reviewdomain.Review, RatingSummary, error)
	ListForUser(ctx context.Context, userID uuid.UUID) ([]*reviewdomain.Review, RatingSummary, error)
}
