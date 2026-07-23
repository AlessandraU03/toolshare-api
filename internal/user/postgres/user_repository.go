package userpostgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	apperrors "github.com/yourusername/tool-inventory-api/internal/shared/errors"
	userdomain "github.com/yourusername/tool-inventory-api/internal/user/domain"
	userports "github.com/yourusername/tool-inventory-api/internal/user/ports"
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

func (r *UserRepository) GetMPCustomerID(ctx context.Context, id uuid.UUID) (string, error) {
	var customerID *string
	err := r.db.QueryRow(ctx, `SELECT mp_customer_id FROM users WHERE id = $1`, id).Scan(&customerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", apperrors.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("consultar mp_customer_id: %w", err)
	}
	if customerID == nil {
		return "", nil
	}
	return *customerID, nil
}

func (r *UserRepository) SetMPCustomerID(ctx context.Context, id uuid.UUID, customerID string) error {
	tag, err := r.db.Exec(ctx, `UPDATE users SET mp_customer_id = $1, updated_at = now() WHERE id = $2`, customerID, id)
	if err != nil {
		return fmt.Errorf("guardar mp_customer_id: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

// GetMPSellerAccount devuelve la cuenta de Mercado Pago que el propietario
// vinculó vía OAuth (Marketplace), o SellerUserID vacío si no ha conectado.
func (r *UserRepository) GetMPSellerAccount(ctx context.Context, id uuid.UUID) (*userdomain.MPSellerAccount, error) {
	var sellerUserID, accessToken, refreshToken *string
	var expiresAt *time.Time
	err := r.db.QueryRow(ctx,
		`SELECT mp_seller_user_id, mp_seller_access_token, mp_seller_refresh_token, mp_seller_token_expires_at
		 FROM users WHERE id = $1`, id).
		Scan(&sellerUserID, &accessToken, &refreshToken, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("consultar cuenta MP del vendedor: %w", err)
	}

	account := &userdomain.MPSellerAccount{}
	if sellerUserID != nil {
		account.SellerUserID = *sellerUserID
	}
	if accessToken != nil {
		account.AccessToken = *accessToken
	}
	if refreshToken != nil {
		account.RefreshToken = *refreshToken
	}
	if expiresAt != nil {
		account.ExpiresAt = *expiresAt
	}
	return account, nil
}

// SetMPSellerAccount guarda (o actualiza) el vínculo OAuth con la cuenta de
// Mercado Pago del propietario.
func (r *UserRepository) SetMPSellerAccount(ctx context.Context, id uuid.UUID, sellerUserID, accessToken, refreshToken string, expiresAt time.Time) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE users SET mp_seller_user_id = $1, mp_seller_access_token = $2,
		 mp_seller_refresh_token = $3, mp_seller_token_expires_at = $4, updated_at = now()
		 WHERE id = $5`,
		sellerUserID, accessToken, refreshToken, expiresAt, id)
	if err != nil {
		return fmt.Errorf("guardar cuenta MP del vendedor: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

// GetBankAccount devuelve los datos bancarios que el propietario registró
// para recibir pagos manuales de disputas ganadas con seguro activo.
func (r *UserRepository) GetBankAccount(ctx context.Context, id uuid.UUID) (*userdomain.BankAccount, error) {
	var clabe, holder, bank *string
	err := r.db.QueryRow(ctx,
		`SELECT bank_clabe, bank_account_holder, bank_name FROM users WHERE id = $1`, id).
		Scan(&clabe, &holder, &bank)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("consultar datos bancarios: %w", err)
	}

	account := &userdomain.BankAccount{}
	if clabe != nil {
		account.CLABE = *clabe
	}
	if holder != nil {
		account.AccountHolder = *holder
	}
	if bank != nil {
		account.BankName = *bank
	}
	return account, nil
}

// SaveBankAccount guarda (o actualiza) los datos bancarios del propietario.
func (r *UserRepository) SaveBankAccount(ctx context.Context, id uuid.UUID, account userdomain.BankAccount) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE users SET bank_clabe = $1, bank_account_holder = $2, bank_name = $3, updated_at = now()
		 WHERE id = $4`,
		account.CLABE, account.AccountHolder, account.BankName, id)
	if err != nil {
		return fmt.Errorf("guardar datos bancarios: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *UserRepository) SaveCard(ctx context.Context, card *userdomain.SavedCard) (*userdomain.SavedCard, error) {
	query := `
		INSERT INTO saved_cards (user_id, mp_card_id, card_brand, last_four_digits, expiration_month, expiration_year)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, user_id, mp_card_id, card_brand, last_four_digits, expiration_month, expiration_year, created_at
	`
	saved := &userdomain.SavedCard{}
	err := r.db.QueryRow(ctx, query,
		card.UserID, card.MPCardID, card.CardBrand, card.LastFourDigits, card.ExpirationMonth, card.ExpirationYear,
	).Scan(&saved.ID, &saved.UserID, &saved.MPCardID, &saved.CardBrand, &saved.LastFourDigits, &saved.ExpirationMonth, &saved.ExpirationYear, &saved.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("guardar tarjeta: %w", err)
	}
	return saved, nil
}

func (r *UserRepository) ListCards(ctx context.Context, userID uuid.UUID) ([]*userdomain.SavedCard, error) {
	query := `
		SELECT id, user_id, mp_card_id, card_brand, last_four_digits, expiration_month, expiration_year, created_at
		FROM saved_cards WHERE user_id = $1 ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("listar tarjetas: %w", err)
	}
	defer rows.Close()

	var cards []*userdomain.SavedCard
	for rows.Next() {
		c := &userdomain.SavedCard{}
		if err := rows.Scan(&c.ID, &c.UserID, &c.MPCardID, &c.CardBrand, &c.LastFourDigits, &c.ExpirationMonth, &c.ExpirationYear, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("escanear tarjeta: %w", err)
		}
		cards = append(cards, c)
	}
	return cards, rows.Err()
}

func (r *UserRepository) DeleteCard(ctx context.Context, userID uuid.UUID, cardID uuid.UUID) (string, error) {
	var mpCardID string
	err := r.db.QueryRow(ctx,
		`DELETE FROM saved_cards WHERE id = $1 AND user_id = $2 RETURNING mp_card_id`,
		cardID, userID,
	).Scan(&mpCardID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", apperrors.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("eliminar tarjeta: %w", err)
	}
	return mpCardID, nil
}
