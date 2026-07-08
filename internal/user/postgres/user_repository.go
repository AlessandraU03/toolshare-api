package userpostgres

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	userdomain "github.com/yourusername/tool-inventory-api/internal/user/domain"
	userports "github.com/yourusername/tool-inventory-api/internal/user/ports"
	apperrors "github.com/yourusername/tool-inventory-api/internal/shared/errors"
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) userports.UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *userdomain.User) (*userdomain.User, error) {
	query := `
		INSERT INTO users (name, email, password, role, is_pro, phone, ine)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, name, email, role, is_pro, phone, ine, created_at, updated_at
	`
	created := &userdomain.User{}
	err := r.db.QueryRow(ctx, query, user.Name, user.Email, user.Password, user.Role, user.IsPro, user.Phone, user.INE).
		Scan(&created.ID, &created.Name, &created.Email, &created.Role, &created.IsPro, &created.Phone, &created.INE, &created.CreatedAt, &created.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("crear usuario: %w", err)
	}
	return created, nil
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*userdomain.User, error) {
	query := `SELECT id, name, email, password, role, is_pro, phone, ine, created_at, updated_at FROM users WHERE email = $1`
	user := &userdomain.User{}
	err := r.db.QueryRow(ctx, query, email).
		Scan(&user.ID, &user.Name, &user.Email, &user.Password, &user.Role, &user.IsPro, &user.Phone, &user.INE, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("buscar usuario por email: %w", err)
	}
	return user, nil
}

func (r *UserRepository) FindByID(ctx context.Context, id uuid.UUID) (*userdomain.User, error) {
	query := `SELECT id, name, email, role, is_pro, phone, ine, created_at, updated_at FROM users WHERE id = $1`
	user := &userdomain.User{}
	err := r.db.QueryRow(ctx, query, id).
		Scan(&user.ID, &user.Name, &user.Email, &user.Role, &user.IsPro, &user.Phone, &user.INE, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("buscar usuario por ID: %w", err)
	}
	return user, nil
}

func (r *UserRepository) UpdateIsPro(ctx context.Context, id uuid.UUID, isPro bool) error {
	tag, err := r.db.Exec(ctx, `UPDATE users SET is_pro = $1, updated_at = now() WHERE id = $2`, isPro, id)
	if err != nil {
		return fmt.Errorf("actualizar is_pro del usuario: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}
