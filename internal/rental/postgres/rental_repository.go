package rentalpostgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	apperrors "github.com/yourusername/tool-inventory-api/internal/shared/errors"
	rentaldomain "github.com/yourusername/tool-inventory-api/internal/rental/domain"
	rentalports "github.com/yourusername/tool-inventory-api/internal/rental/ports"
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
	daily_rate, total_amount, status,
	owner_confirmed_delivery, requester_confirmed_delivery,
	requester_confirmed_return, owner_confirmed_return,
	mp_payment_id, payment_status, deductible_amount,
	contract_hash, delivery_lat, delivery_lng, delivery_at,
	dispute_reason,
	created_at, updated_at`

func (r *RentalRepository) Create(ctx context.Context, rental *rentaldomain.Rental) (*rentaldomain.Rental, error) {
	query := `
		INSERT INTO rentals (
			tool_id, requester_id, owner_id, start_date, end_date,
			daily_rate, total_amount, status,
			owner_confirmed_delivery, requester_confirmed_delivery,
			requester_confirmed_return, owner_confirmed_return,
			mp_payment_id, payment_status, deductible_amount
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,
		          NULLIF($13,''), NULLIF($14,''), $15)
		RETURNING ` + rentalCols

	return r.scanRental(r.db.QueryRow(ctx, query,
		rental.ToolID, rental.RequesterID, rental.OwnerID,
		rental.StartDate, rental.EndDate,
		rental.DailyRate, rental.TotalAmount, rental.Status,
		rental.OwnerConfirmedDelivery, rental.RequesterConfirmedDelivery,
		rental.RequesterConfirmedReturn, rental.OwnerConfirmedReturn,
		rental.MPPaymentID, rental.PaymentStatus, rental.DeductibleAmount,
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
		&rental.DailyRate, &rental.TotalAmount, &rental.Status,
		&rental.OwnerConfirmedDelivery, &rental.RequesterConfirmedDelivery,
		&rental.RequesterConfirmedReturn, &rental.OwnerConfirmedReturn,
		&mpPaymentID, &paymentStatus, &rental.DeductibleAmount,
		&contractHash, &deliveryLat, &deliveryLng, &deliveryAt,
		&disputeReason,
		&rental.CreatedAt, &rental.UpdatedAt,
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
