package toolpostgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	apperrors "github.com/yourusername/tool-inventory-api/internal/shared/errors"
	tooldomain "github.com/yourusername/tool-inventory-api/internal/tool/domain"
	toolports "github.com/yourusername/tool-inventory-api/internal/tool/ports"
)

type ToolRepository struct {
	db *pgxpool.Pool
}

func NewToolRepository(db *pgxpool.Pool) toolports.ToolRepository {
	return &ToolRepository{db: db}
}

func (r *ToolRepository) Create(ctx context.Context, tool *tooldomain.Tool) (*tooldomain.Tool, error) {
	query := `
		INSERT INTO tools (owner_id, name, description, category, photo_url, estimated_value, daily_rate, latitude, longitude, is_available)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, owner_id, name, description, category, photo_url, estimated_value, daily_rate, latitude, longitude, is_available, created_at, updated_at
	`
	return r.scanTool(r.db.QueryRow(ctx, query,
		tool.OwnerID, tool.Name, tool.Description, tool.Category,
		tool.PhotoURL, tool.EstimatedValue, tool.DailyRate, tool.Latitude, tool.Longitude, tool.IsAvailable,
	))
}

func (r *ToolRepository) FindByID(ctx context.Context, id uuid.UUID) (*tooldomain.Tool, error) {
	query := `
		SELECT id, owner_id, name, description, category, photo_url, estimated_value, daily_rate, latitude, longitude, is_available, created_at, updated_at
		FROM tools WHERE id = $1
	`
	t, err := r.scanTool(r.db.QueryRow(ctx, query, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	return t, err
}

func (r *ToolRepository) FindAll(ctx context.Context, filter toolports.ToolFilter) ([]*tooldomain.Tool, error) {
	var conditions []string
	var args []interface{}
	idx := 1

	if filter.OnlyAvailable {
		conditions = append(conditions, fmt.Sprintf("is_available = $%d", idx))
		args = append(args, true)
		idx++
	}
	if filter.Category != "" {
		conditions = append(conditions, fmt.Sprintf("LOWER(category) = LOWER($%d)", idx))
		args = append(args, filter.Category)
		idx++
	}
	if filter.Search != "" {
		conditions = append(conditions, fmt.Sprintf("(LOWER(name) LIKE LOWER($%d) OR LOWER(description) LIKE LOWER($%d))", idx, idx))
		args = append(args, "%"+filter.Search+"%")
		idx++
	}
	if filter.OwnerID != nil {
		conditions = append(conditions, fmt.Sprintf("owner_id = $%d", idx))
		args = append(args, *filter.OwnerID)
		idx++
	}

	query := `
		SELECT id, owner_id, name, description, category, photo_url, estimated_value, daily_rate, latitude, longitude, is_available, created_at, updated_at
		FROM tools
	`
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY name ASC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listar herramientas: %w", err)
	}
	defer rows.Close()

	var tools []*tooldomain.Tool
	for rows.Next() {
		tool := &tooldomain.Tool{}
		if err := rows.Scan(
			&tool.ID, &tool.OwnerID, &tool.Name, &tool.Description, &tool.Category,
			&tool.PhotoURL, &tool.EstimatedValue, &tool.DailyRate, &tool.Latitude, &tool.Longitude, &tool.IsAvailable,
			&tool.CreatedAt, &tool.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("escanear herramienta: %w", err)
		}
		tools = append(tools, tool)
	}
	return tools, rows.Err()
}

func (r *ToolRepository) Update(ctx context.Context, tool *tooldomain.Tool) (*tooldomain.Tool, error) {
	query := `
		UPDATE tools
		SET name = $1, description = $2, category = $3, photo_url = $4,
		    estimated_value = $5, daily_rate = $6, latitude = $7, longitude = $8, is_available = $9
		WHERE id = $10
		RETURNING id, owner_id, name, description, category, photo_url, estimated_value, daily_rate, latitude, longitude, is_available, created_at, updated_at
	`
	return r.scanTool(r.db.QueryRow(ctx, query,
		tool.Name, tool.Description, tool.Category, tool.PhotoURL,
		tool.EstimatedValue, tool.DailyRate, tool.Latitude, tool.Longitude, tool.IsAvailable, tool.ID,
	))
}

func (r *ToolRepository) Delete(ctx context.Context, id uuid.UUID) error {
	var ongoingCount int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM rentals WHERE tool_id = $1 AND status IN ('pending', 'active', 'disputed')`, id).Scan(&ongoingCount)
	if err != nil {
		return fmt.Errorf("verificar rentas activas: %w", err)
	}
	if ongoingCount > 0 {
		return errors.New("renta_en_curso")
	}

	_, err = r.db.Exec(ctx, `DELETE FROM rentals WHERE tool_id = $1`, id)
	if err != nil {
		return fmt.Errorf("eliminar historial de rentas: %w", err)
	}

	result, err := r.db.Exec(ctx, `DELETE FROM tools WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("eliminar herramienta: %w", err)
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *ToolRepository) SetAvailability(ctx context.Context, id uuid.UUID, available bool) error {
	_, err := r.db.Exec(ctx, `UPDATE tools SET is_available = $1 WHERE id = $2`, available, id)
	return err
}

func (r *ToolRepository) scanTool(row pgx.Row) (*tooldomain.Tool, error) {
	tool := &tooldomain.Tool{}
	err := row.Scan(
		&tool.ID, &tool.OwnerID, &tool.Name, &tool.Description, &tool.Category,
		&tool.PhotoURL, &tool.EstimatedValue, &tool.DailyRate, &tool.Latitude, &tool.Longitude, &tool.IsAvailable,
		&tool.CreatedAt, &tool.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("escanear herramienta: %w", err)
	}
	return tool, nil
}
