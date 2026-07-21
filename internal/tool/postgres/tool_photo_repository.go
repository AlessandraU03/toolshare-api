package toolpostgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	tooldomain "github.com/yourusername/tool-inventory-api/internal/tool/domain"
	toolports "github.com/yourusername/tool-inventory-api/internal/tool/ports"
)

type ToolPhotoRepository struct {
	db *pgxpool.Pool
}

func NewToolPhotoRepository(db *pgxpool.Pool) toolports.ToolPhotoRepository {
	return &ToolPhotoRepository{db: db}
}

const toolPhotoCols = `id, tool_id, photo_url, condition_score, created_at`

func (r *ToolPhotoRepository) Create(ctx context.Context, photo *tooldomain.ToolPhoto) (*tooldomain.ToolPhoto, error) {
	query := `
		INSERT INTO tool_photos (tool_id, photo_url, condition_score)
		VALUES ($1, $2, $3)
		RETURNING ` + toolPhotoCols
	row := r.db.QueryRow(ctx, query, photo.ToolID, photo.PhotoURL, photo.ConditionScore)
	return scanToolPhoto(row)
}

func (r *ToolPhotoRepository) FindByToolID(ctx context.Context, toolID uuid.UUID) ([]*tooldomain.ToolPhoto, error) {
	query := `SELECT ` + toolPhotoCols + ` FROM tool_photos WHERE tool_id = $1 ORDER BY created_at ASC`
	rows, err := r.db.Query(ctx, query, toolID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var photos []*tooldomain.ToolPhoto
	for rows.Next() {
		p, err := scanToolPhoto(rows)
		if err != nil {
			return nil, err
		}
		photos = append(photos, p)
	}
	return photos, rows.Err()
}

type photoRowScanner interface {
	Scan(dest ...any) error
}

func scanToolPhoto(row photoRowScanner) (*tooldomain.ToolPhoto, error) {
	var p tooldomain.ToolPhoto
	if err := row.Scan(&p.ID, &p.ToolID, &p.PhotoURL, &p.ConditionScore, &p.CreatedAt); err != nil {
		return nil, err
	}
	return &p, nil
}
