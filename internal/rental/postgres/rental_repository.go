package rentalpostgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	rentaldomain "github.com/yourusername/tool-inventory-api/internal/rental/domain"
	rentalports "github.com/yourusername/tool-inventory-api/internal/rental/ports"
	apperrors "github.com/yourusername/tool-inventory-api/internal/shared/errors"
)

type RentalRepository struct {
	db *pgxpool.Pool
}

func NewRentalRepository(db *pgxpool.Pool) rentalports.RentalRepository {
	return &RentalRepository{db: db}
}

// ── columnas seleccionadas en todos los SELECT ────────────────────────────────
const rentalCols = `
	id, tool_id, requester_id, owner_id, start_date, end_date,
	daily_rate, total_amount, status, payment_method,
	owner_confirmed_delivery, requester_confirmed_delivery,
	requester_confirmed_return, owner_confirmed_return,
	mp_payment_id, payment_status, deductible_amount, commission_amount,
	contract_hash, delivery_lat, delivery_lng, delivery_at,
	dispute_reason,
	created_at, updated_at,
	COALESCE((SELECT name FROM users WHERE id = owner_id), '') AS owner_name,
	COALESCE((SELECT name FROM users WHERE id = requester_id), '') AS requester_name`

func (r *RentalRepository) Create(ctx context.Context, rental *rentaldomain.Rental) (*rentaldomain.Rental, error) {
	if rental.PaymentMethod == "" {
		rental.PaymentMethod = "card"
	}
	query := `
		INSERT INTO rentals (
			tool_id, requester_id, owner_id, start_date, end_date,
			daily_rate, total_amount, status, payment_method,
			owner_confirmed_delivery, requester_confirmed_delivery,
			requester_confirmed_return, owner_confirmed_return,
			mp_payment_id, payment_status, deductible_amount, commission_amount
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,
		          NULLIF($14,''), NULLIF($15,''), $16, $17)
		RETURNING ` + rentalCols

	return r.scanRental(r.db.QueryRow(ctx, query,
		rental.ToolID, rental.RequesterID, rental.OwnerID,
		rental.StartDate, rental.EndDate,
		rental.DailyRate, rental.TotalAmount, rental.Status, rental.PaymentMethod,
		rental.OwnerConfirmedDelivery, rental.RequesterConfirmedDelivery,
		rental.RequesterConfirmedReturn, rental.OwnerConfirmedReturn,
		rental.MPPaymentID, rental.PaymentStatus, rental.DeductibleAmount, rental.CommissionAmount,
	))
}

func (r *RentalRepository) FindByID(ctx context.Context, id uuid.UUID) (*rentaldomain.Rental, error) {
	query := `SELECT ` + rentalCols + ` FROM rentals WHERE id = $1`
	rental, err := r.scanRental(r.db.QueryRow(ctx, query, id))
	if errors.Is(err, apperrors.ErrNotFound) {
		return nil, apperrors.ErrNotFound
	}
	return rental, err
}

func (r *RentalRepository) FindByUser(ctx context.Context, userID uuid.UUID) ([]*rentaldomain.Rental, error) {
	query := `
		SELECT ` + rentalCols + `
		FROM rentals
		WHERE requester_id = $1 OR owner_id = $1
		ORDER BY created_at DESC`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("listar rentas: %w", err)
	}
	defer rows.Close()

	var rentals []*rentaldomain.Rental
	for rows.Next() {
		rental := &rentaldomain.Rental{}
		if err := r.scanRow(rows, rental); err != nil {
			return nil, err
		}
		rentals = append(rentals, rental)
	}
	return rentals, rows.Err()
}

func (r *RentalRepository) FindAll(ctx context.Context, status string) ([]*rentaldomain.Rental, error) {
	query := `SELECT ` + rentalCols + ` FROM rentals`
	var args []interface{}
	if status != "" {
		query += ` WHERE status = $1`
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listar todas las rentas: %w", err)
	}
	defer rows.Close()

	var rentals []*rentaldomain.Rental
	for rows.Next() {
		rental := &rentaldomain.Rental{}
		if err := r.scanRow(rows, rental); err != nil {
			return nil, err
		}
		rentals = append(rentals, rental)
	}
	return rentals, rows.Err()
}

func (r *RentalRepository) GetAdminStats(ctx context.Context) (int, int, int, float64, error) {
	query := `
		SELECT 
			COUNT(*)::int,
			COUNT(*) FILTER (WHERE status = 'active')::int,
			COUNT(*) FILTER (WHERE status = 'disputed')::int,
			COALESCE(SUM(total_amount + deductible_amount + commission_amount) FILTER (WHERE payment_status = 'authorized'), 0)::float8
		FROM rentals`
	var total, active, disputed int
	var frozen float64
	err := r.db.QueryRow(ctx, query).Scan(&total, &active, &disputed, &frozen)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("consultar estadísticas admin: %w", err)
	}
	return total, active, disputed, frozen, nil
}

func (r *RentalRepository) Update(ctx context.Context, rental *rentaldomain.Rental) (*rentaldomain.Rental, error) {
	query := `
		UPDATE rentals
		SET status                      = $1,
		    owner_confirmed_delivery     = $2,
		    requester_confirmed_delivery = $3,
		    requester_confirmed_return   = $4,
		    owner_confirmed_return       = $5,
		    mp_payment_id               = NULLIF($6,''),
		    payment_status              = NULLIF($7,''),
		    deductible_amount           = $8,
		    contract_hash               = NULLIF($9,''),
		    delivery_lat                = $10,
		    delivery_lng                = $11,
		    delivery_at                 = $12,
		    dispute_reason              = NULLIF($13,'')
		WHERE id = $14
		RETURNING ` + rentalCols

	// delivery_lat/lng: use nil when zero to keep NULL in DB
	var lat, lng interface{}
	if rental.DeliveryLat != 0 || rental.DeliveryLng != 0 {
		lat = rental.DeliveryLat
		lng = rental.DeliveryLng
	}

	return r.scanRental(r.db.QueryRow(ctx, query,
		rental.Status,
		rental.OwnerConfirmedDelivery, rental.RequesterConfirmedDelivery,
		rental.RequesterConfirmedReturn, rental.OwnerConfirmedReturn,
		rental.MPPaymentID, rental.PaymentStatus, rental.DeductibleAmount,
		rental.ContractHash,
		lat, lng, rental.DeliveryAt,
		rental.DisputeReason,
		rental.ID,
	))
}

// ── helpers de scan ───────────────────────────────────────────────────────────

func (r *RentalRepository) scanRental(row pgx.Row) (*rentaldomain.Rental, error) {
	rental := &rentaldomain.Rental{}
	err := r.scan(func(dest ...any) error { return row.Scan(dest...) }, rental)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("escanear renta: %w", err)
	}
	return rental, nil
}

func (r *RentalRepository) scanRow(rows pgx.Rows, rental *rentaldomain.Rental) error {
	return r.scan(func(dest ...any) error { return rows.Scan(dest...) }, rental)
}

// scan centraliza la lectura de columnas para evitar duplicación.
func (r *RentalRepository) scan(scanFn func(...any) error, rental *rentaldomain.Rental) error {
	var (
		mpPaymentID, paymentStatus, contractHash, disputeReason *string
		deliveryLat, deliveryLng                                *float64
		deliveryAt                                              *time.Time
	)

	err := scanFn(
		&rental.ID, &rental.ToolID, &rental.RequesterID, &rental.OwnerID,
		&rental.StartDate, &rental.EndDate,
		&rental.DailyRate, &rental.TotalAmount, &rental.Status, &rental.PaymentMethod,
		&rental.OwnerConfirmedDelivery, &rental.RequesterConfirmedDelivery,
		&rental.RequesterConfirmedReturn, &rental.OwnerConfirmedReturn,
		&mpPaymentID, &paymentStatus, &rental.DeductibleAmount, &rental.CommissionAmount,
		&contractHash, &deliveryLat, &deliveryLng, &deliveryAt,
		&disputeReason,
		&rental.CreatedAt, &rental.UpdatedAt,
		&rental.OwnerName, &rental.RequesterName,
	)
	if err != nil {
		return err
	}

	if mpPaymentID != nil {
		rental.MPPaymentID = *mpPaymentID
	}
	if paymentStatus != nil {
		rental.PaymentStatus = *paymentStatus
	}
	if contractHash != nil {
		rental.ContractHash = *contractHash
	}
	if deliveryLat != nil {
		rental.DeliveryLat = *deliveryLat
	}
	if deliveryLng != nil {
		rental.DeliveryLng = *deliveryLng
	}
	rental.DeliveryAt = deliveryAt
	if disputeReason != nil {
		rental.DisputeReason = *disputeReason
	}
	return nil
}

func (r *RentalRepository) GetMessages(ctx context.Context, rentalID uuid.UUID) ([]*rentaldomain.Message, error) {
	query := `SELECT id, rental_id, sender_id, message, created_at FROM rental_messages WHERE rental_id = $1 ORDER BY created_at ASC`
	rows, err := r.db.Query(ctx, query, rentalID)
	if err != nil {
		return nil, fmt.Errorf("consultar mensajes: %w", err)
	}
	defer rows.Close()

	var msgs []*rentaldomain.Message
	for rows.Next() {
		m := &rentaldomain.Message{}
		if err := rows.Scan(&m.ID, &m.RentalID, &m.SenderID, &m.Message, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func (r *RentalRepository) CreateMessage(ctx context.Context, msg *rentaldomain.Message) (*rentaldomain.Message, error) {
	query := `INSERT INTO rental_messages (rental_id, sender_id, message) VALUES ($1, $2, $3) RETURNING id, rental_id, sender_id, message, created_at`
	m := &rentaldomain.Message{}
	err := r.db.QueryRow(ctx, query, msg.RentalID, msg.SenderID, msg.Message).Scan(&m.ID, &m.RentalID, &m.SenderID, &m.Message, &m.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("crear mensaje: %w", err)
	}
	return m, nil
}

func (r *RentalRepository) LogFingerprint(ctx context.Context, userID uuid.UUID, ipAddress, deviceID string) error {
	query := `
		INSERT INTO user_fingerprints (user_id, ip_address, device_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, ip_address, device_id) DO NOTHING
	`
	_, err := r.db.Exec(ctx, query, userID, ipAddress, deviceID)
	return err
}

func (r *RentalRepository) CheckCollusion(ctx context.Context, ownerID, requesterID uuid.UUID) (bool, error) {
	// Si es el mismo usuario, es colusión automática de autorenta
	if ownerID == requesterID {
		return true, nil
	}
	// Buscar coincidencia de IP o Device ID
	query := `
		SELECT EXISTS (
			SELECT 1 
			FROM user_fingerprints f1
			JOIN user_fingerprints f2 ON f1.ip_address = f2.ip_address OR f1.device_id = f2.device_id
			WHERE f1.user_id = $1 AND f2.user_id = $2
		)
	`
	var exists bool
	err := r.db.QueryRow(ctx, query, ownerID, requesterID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}
