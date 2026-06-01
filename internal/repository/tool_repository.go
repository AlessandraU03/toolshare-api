package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yourusername/tool-inventory-api/internal/model"
)

// ToolRepository maneja todas las operaciones de base de datos para herramientas.
type ToolRepository struct {
	db *pgxpool.Pool
}

// NewToolRepository crea una nueva instancia del repositorio de herramientas.
func NewToolRepository(db *pgxpool.Pool) *ToolRepository {
	return &ToolRepository{db: db}
}

// FindAll retorna todas las herramientas del catálogo.
// Acepta un filtro opcional por disponibilidad (nil = sin filtro).
func (r *ToolRepository) FindAll(ctx context.Context, onlyAvailable bool) ([]*model.Tool, error) {
	var query string
	var args []interface{}

	if onlyAvailable {
		query = `
			SELECT id, owner_id, name, description, category, is_available, created_at, updated_at
			FROM tools
			WHERE is_available = TRUE
			ORDER BY name ASC
		`
	} else {
		query = `
			SELECT id, owner_id, name, description, category, is_available, created_at, updated_at
			FROM tools
			ORDER BY name ASC
		`
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("error al consultar herramientas: %w", err)
	}
	defer rows.Close()

	tools := make([]*model.Tool, 0)
	for rows.Next() {
		tool := &model.Tool{}
		if err := rows.Scan(
			&tool.ID,
			&tool.OwnerID,
			&tool.Name,
			&tool.Description,
			&tool.Category,
			&tool.IsAvailable,
			&tool.CreatedAt,
			&tool.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("error al escanear herramienta: %w", err)
		}
		tools = append(tools, tool)
	}

	return tools, rows.Err()
}

// FindByID busca una herramienta por su UUID.
func (r *ToolRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Tool, error) {
	query := `
		SELECT id, owner_id, name, description, category, is_available, created_at, updated_at
		FROM tools
		WHERE id = $1
	`
	tool := &model.Tool{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&tool.ID,
		&tool.OwnerID,
		&tool.Name,
		&tool.Description,
		&tool.Category,
		&tool.IsAvailable,
		&tool.CreatedAt,
		&tool.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("error al buscar herramienta por ID: %w", err)
	}
	return tool, nil
}

// Create inserta una nueva herramienta en la base de datos.
func (r *ToolRepository) Create(ctx context.Context, tool *model.Tool) (*model.Tool, error) {
	query := `
		INSERT INTO tools (owner_id, name, description, category, is_available)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, owner_id, name, description, category, is_available, created_at, updated_at
	`
	created := &model.Tool{}
	err := r.db.QueryRow(ctx, query,
		tool.OwnerID,
		tool.Name,
		tool.Description,
		tool.Category,
		tool.IsAvailable,
	).Scan(
		&created.ID,
		&created.OwnerID,
		&created.Name,
		&created.Description,
		&created.Category,
		&created.IsAvailable,
		&created.CreatedAt,
		&created.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("error al crear herramienta: %w", err)
	}
	return created, nil
}

// Update modifica una herramienta existente. Solo actualiza los campos no nulos.
// Verifica que el ownerID coincida para prevenir que un propietario edite
// herramientas de otro propietario.
func (r *ToolRepository) Update(ctx context.Context, id uuid.UUID, ownerID uuid.UUID, req *model.UpdateToolRequest) (*model.Tool, error) {
	// Primero verificar que la herramienta existe y pertenece al propietario
	existing, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.OwnerID != ownerID {
		return nil, ErrForbidden
	}

	// Aplicar los cambios solo a los campos enviados en el request
	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Description != nil {
		existing.Description = *req.Description
	}
	if req.Category != nil {
		existing.Category = *req.Category
	}
	if req.IsAvailable != nil {
		existing.IsAvailable = *req.IsAvailable
	}

	query := `
		UPDATE tools
		SET name = $1, description = $2, category = $3, is_available = $4
		WHERE id = $5
		RETURNING id, owner_id, name, description, category, is_available, created_at, updated_at
	`
	updated := &model.Tool{}
	err = r.db.QueryRow(ctx, query,
		existing.Name,
		existing.Description,
		existing.Category,
		existing.IsAvailable,
		id,
	).Scan(
		&updated.ID,
		&updated.OwnerID,
		&updated.Name,
		&updated.Description,
		&updated.Category,
		&updated.IsAvailable,
		&updated.CreatedAt,
		&updated.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("error al actualizar herramienta: %w", err)
	}
	return updated, nil
}

// Delete elimina una herramienta. Verifica que el ownerID sea el dueño.
func (r *ToolRepository) Delete(ctx context.Context, id uuid.UUID, ownerID uuid.UUID) error {
	// Verificar propiedad antes de eliminar
	existing, err := r.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if existing.OwnerID != ownerID {
		return ErrForbidden
	}

	query := `DELETE FROM tools WHERE id = $1`
	result, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("error al eliminar herramienta: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ErrForbidden indica que el usuario no tiene permiso sobre el recurso.
var ErrForbidden = errors.New("no tienes permiso sobre este recurso")
