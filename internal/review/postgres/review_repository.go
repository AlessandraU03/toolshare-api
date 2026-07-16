package reviewpostgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	reviewdomain "github.com/yourusername/tool-inventory-api/internal/review/domain"
	reviewports "github.com/yourusername/tool-inventory-api/internal/review/ports"
)

type ReviewRepository struct {
	db *pgxpool.Pool
}

func NewReviewRepository(db *pgxpool.Pool) reviewports.ReviewRepository {
	return &ReviewRepository{db: db}
}

func (r *ReviewRepository) Create(ctx context.Context, review *reviewdomain.Review) (*reviewdomain.Review, error) {
	query := `
		INSERT INTO reviews (rental_id, author_id, target_type, target_id, rating, comment)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, rental_id, author_id, target_type, target_id, rating, comment, created_at
	`
	return scanReview(r.db.QueryRow(ctx, query,
		review.RentalID, review.AuthorID, review.TargetType, review.TargetID, review.Rating, review.Comment,
	))
}

func (r *ReviewRepository) ExistsByRentalAndAuthor(ctx context.Context, rentalID, authorID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM reviews WHERE rental_id = $1 AND author_id = $2)`,
		rentalID, authorID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("verificar reseña existente: %w", err)
	}
	return exists, nil
}

func (r *ReviewRepository) FindByTarget(ctx context.Context, targetType reviewdomain.TargetType, targetID uuid.UUID) ([]*reviewdomain.Review, error) {
	query := `
		SELECT id, rental_id, author_id, target_type, target_id, rating, comment, created_at
		FROM reviews
		WHERE target_type = $1 AND target_id = $2
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, targetType, targetID)
	if err != nil {
		return nil, fmt.Errorf("listar reseñas: %w", err)
	}
	defer rows.Close()

	var reviews []*reviewdomain.Review
	for rows.Next() {
		review, err := scanReview(rows)
		if err != nil {
			return nil, err
		}
		reviews = append(reviews, review)
	}
	return reviews, rows.Err()
}

func (r *ReviewRepository) GetSummary(ctx context.Context, targetType reviewdomain.TargetType, targetID uuid.UUID) (reviewports.RatingSummary, error) {
	var summary reviewports.RatingSummary
	err := r.db.QueryRow(ctx,
		`SELECT COALESCE(AVG(rating), 0), COUNT(*) FROM reviews WHERE target_type = $1 AND target_id = $2`,
		targetType, targetID,
	).Scan(&summary.Average, &summary.Count)
	if err != nil {
		return reviewports.RatingSummary{}, fmt.Errorf("calcular resumen de calificación: %w", err)
	}
	return summary, nil
}

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanReview(row rowScanner) (*reviewdomain.Review, error) {
	review := &reviewdomain.Review{}
	err := row.Scan(
		&review.ID, &review.RentalID, &review.AuthorID, &review.TargetType,
		&review.TargetID, &review.Rating, &review.Comment, &review.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("escanear reseña: %w", err)
	}
	return review, nil
}
