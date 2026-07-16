package reviewhandler

import (
	"time"

	reviewdomain "github.com/yourusername/tool-inventory-api/internal/review/domain"
	reviewports "github.com/yourusername/tool-inventory-api/internal/review/ports"
)

type CreateReviewRequest struct {
	Rating  int    `json:"rating"  binding:"required,min=1,max=5" example:"5"`
	Comment string `json:"comment" example:"Herramienta en excelente estado, tal cual la descripción"`
}

type ReviewResponse struct {
	ID         string `json:"id"`
	RentalID   string `json:"rental_id"`
	AuthorID   string `json:"author_id"`
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Rating     int    `json:"rating"`
	Comment    string `json:"comment"`
	CreatedAt  string `json:"created_at"`
}

type ReviewListResponse struct {
	Reviews       []ReviewResponse `json:"reviews"`
	AverageRating float64          `json:"average_rating"`
	ReviewCount   int              `json:"review_count"`
}

func ToReviewResponse(r *reviewdomain.Review) ReviewResponse {
	return ReviewResponse{
		ID:         r.ID.String(),
		RentalID:   r.RentalID.String(),
		AuthorID:   r.AuthorID.String(),
		TargetType: string(r.TargetType),
		TargetID:   r.TargetID.String(),
		Rating:     r.Rating,
		Comment:    r.Comment,
		CreatedAt:  r.CreatedAt.Format(time.RFC3339),
	}
}

func ToReviewListResponse(reviews []*reviewdomain.Review, summary reviewports.RatingSummary) ReviewListResponse {
	resp := make([]ReviewResponse, 0, len(reviews))
	for _, r := range reviews {
		resp = append(resp, ToReviewResponse(r))
	}
	return ReviewListResponse{
		Reviews:       resp,
		AverageRating: summary.Average,
		ReviewCount:   summary.Count,
	}
}
