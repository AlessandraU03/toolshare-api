// Package repository contiene la capa de acceso a datos (DAL).
// Todas las queries SQL viven aquí, separadas de la lógica de negocio.
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

// UserRepository maneja todas las operaciones de base de datos para usuarios.
type UserRepository struct {
	db *pgxpool.Pool
}

// NewUserRepository crea una nueva instancia del repositorio de usuarios.
func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

// Create inserta un nuevo usuario en la base de datos.
// Retorna el usuario creado con su ID y timestamps asignados por PostgreSQL.
func (r *UserRepository) Create(ctx context.Context, user *model.User) (*model.User, error) {
	query := `
		INSERT INTO users (name, email, password, role, phone, ine_number)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, name, email, role, phone, ine_number, created_at, updated_at
	`
	created := &model.User{}
	err := r.db.QueryRow(ctx, query,
		user.Name,
		user.Email,
		user.Password,
		user.Role,
		user.Phone,
		user.IneNumber,
	).Scan(
		&created.ID,
		&created.Name,
		&created.Email,
		&created.Role,
		&created.Phone,
		&created.IneNumber,
		&created.CreatedAt,
		&created.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("error al crear usuario: %w", err)
	}
	return created, nil
}

// FindByEmail busca un usuario por su email.
// Retorna ErrNotFound si no existe, para distinguirlo de errores de BD.
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	query := `
		SELECT id, name, email, password, role, phone, ine_number, created_at, updated_at
		FROM users
		WHERE email = $1
	`
	user := &model.User{}
	err := r.db.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.Password,
		&user.Role,
		&user.Phone,
		&user.IneNumber,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("error al buscar usuario por email: %w", err)
	}
	return user, nil
}

// FindByID busca un usuario por su UUID.
func (r *UserRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	query := `
		SELECT id, name, email, role, phone, ine_number, created_at, updated_at
		FROM users
		WHERE id = $1
	`
	user := &model.User{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.Role,
		&user.Phone,
		&user.IneNumber,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("error al buscar usuario por ID: %w", err)
	}
	return user, nil
}

// ErrNotFound es un error centinela para indicar que el recurso no existe.
// Permite que los handlers distingan "no encontrado" de errores de servidor.
var ErrNotFound = errors.New("registro no encontrado")

// ErrDuplicateEmail se retorna cuando se intenta registrar un email ya existente.
var ErrDuplicateEmail = errors.New("el email ya está registrado")
