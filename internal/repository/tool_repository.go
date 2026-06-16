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

// scanTool escanea una fila de la BD en una estructura Tool.
// Centraliza el mapeo columna→campo para evitar errores de orden.
func scanTool(row interface {
	Scan(dest ...any) error
}, tool *model.Tool) error {
	return row.Scan(
		&tool.ID,
		&tool.OwnerID,
		&tool.Name,
		&tool.Description,
		&tool.Category,
		&tool.IsAvailable,
		&tool.Brand,
		&tool.Model,
		&tool.WearLevel,
		&tool.Latitude,
		&tool.Longitude,
		&tool.PricePerDay,
		&tool.PhotoURL,
		&tool.CreatedAt,
		&tool.UpdatedAt,
	)
}

const toolSelectColumns = `
	id, owner_id, name, description, category, is_available,
	brand, model, wear_level, latitude, longitude, price_per_day, photo_url,
	created_at, updated_at
`

// FindAll retorna todas las herramientas del catálogo.
// Acepta un filtro opcional por disponibilidad.
func (r *ToolRepository) FindAll(ctx context.Context, onlyAvailable bool) ([]*model.Tool, error) {
	query := `SELECT ` + toolSelectColumns + ` FROM tools`
	if onlyAvailable {
		query += ` WHERE is_available = TRUE`
	}
	query += ` ORDER BY name ASC`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error al consultar herramientas: %w", err)
	}
	defer rows.Close()

	tools := make([]*model.Tool, 0)
	for rows.Next() {
		tool := &model.Tool{}
		if err := scanTool(rows, tool); err != nil {
			return nil, fmt.Errorf("error al escanear herramienta: %w", err)
		}
		tools = append(tools, tool)
	}

	return tools, rows.Err()
}

// FindByOwnerID retorna todas las herramientas que pertenecen a un propietario específico.
// Usado por GET /api/tools/mine para que cada owner vea solo sus propias herramientas.
func (r *ToolRepository) FindByOwnerID(ctx context.Context, ownerID uuid.UUID) ([]*model.Tool, error) {
	query := `SELECT ` + toolSelectColumns + ` FROM tools WHERE owner_id = $1 ORDER BY created_at DESC`

	rows, err := r.db.Query(ctx, query, ownerID)
	if err != nil {
		return nil, fmt.Errorf("error al consultar herramientas del propietario: %w", err)
	}
	defer rows.Close()

	tools := make([]*model.Tool, 0)
	for rows.Next() {
		tool := &model.Tool{}
		if err := scanTool(rows, tool); err != nil {
			return nil, fmt.Errorf("error al escanear herramienta: %w", err)
		}
		tools = append(tools, tool)
	}

	return tools, rows.Err()
}

// FindByID busca una herramienta por su UUID.
func (r *ToolRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Tool, error) {
	query := `SELECT ` + toolSelectColumns + ` FROM tools WHERE id = $1`
	tool := &model.Tool{}
	err := scanTool(r.db.QueryRow(ctx, query, id), tool)
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
		INSERT INTO tools (
			owner_id, name, description, category, is_available,
			brand, model, wear_level, latitude, longitude, price_per_day
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING ` + toolSelectColumns

	created := &model.Tool{}
	err := scanTool(r.db.QueryRow(ctx, query,
		tool.OwnerID,
		tool.Name,
		tool.Description,
		tool.Category,
		tool.IsAvailable,
		tool.Brand,
		tool.Model,
		tool.WearLevel,
		tool.Latitude,
		tool.Longitude,
		tool.PricePerDay,
	), created)
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
	if req.Brand != nil {
		existing.Brand = req.Brand
	}
	if req.Model != nil {
		existing.Model = req.Model
	}
	if req.WearLevel != nil {
		existing.WearLevel = req.WearLevel
	}
	if req.Latitude != nil {
		existing.Latitude = req.Latitude
	}
	if req.Longitude != nil {
		existing.Longitude = req.Longitude
	}
	if req.PricePerDay != nil {
		existing.PricePerDay = req.PricePerDay
	}

	query := `
		UPDATE tools
		SET name         = $1,
		    description  = $2,
		    category     = $3,
		    is_available = $4,
		    brand        = $5,
		    model        = $6,
		    wear_level   = $7,
		    latitude     = $8,
		    longitude    = $9,
		    price_per_day = $10
		WHERE id = $11
		RETURNING ` + toolSelectColumns

	updated := &model.Tool{}
	err = scanTool(r.db.QueryRow(ctx, query,
		existing.Name,
		existing.Description,
		existing.Category,
		existing.IsAvailable,
		existing.Brand,
		existing.Model,
		existing.WearLevel,
		existing.Latitude,
		existing.Longitude,
		existing.PricePerDay,
		id,
	), updated)
	if err != nil {
		return nil, fmt.Errorf("error al actualizar herramienta: %w", err)
	}
	return updated, nil
}

// UpdatePhoto actualiza la URL de la foto de una herramienta.
// Solo el propietario puede hacerlo.
func (r *ToolRepository) UpdatePhoto(ctx context.Context, id uuid.UUID, ownerID uuid.UUID, photoURL string) (*model.Tool, error) {
	// Verificar propiedad
	existing, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.OwnerID != ownerID {
		return nil, ErrForbidden
	}

	query := `
		UPDATE tools SET photo_url = $1 WHERE id = $2
		RETURNING ` + toolSelectColumns

	updated := &model.Tool{}
	err = scanTool(r.db.QueryRow(ctx, query, photoURL, id), updated)
	if err != nil {
		return nil, fmt.Errorf("error al actualizar foto: %w", err)
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
