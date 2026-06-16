package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yourusername/tool-inventory-api/internal/model"
)

// RentalRepository maneja todas las operaciones de base de datos para rentas.
type RentalRepository struct {
	db *pgxpool.Pool
}

// NewRentalRepository crea una nueva instancia del repositorio de rentas.
func NewRentalRepository(db *pgxpool.Pool) *RentalRepository {
	return &RentalRepository{db: db}
}

const rentalSelectColumns = `
	id, tool_id, requester_id, days, price_per_day, total, deposit,
	status, payment_id, payment_url,
	requester_confirmed_delivery, owner_confirmed_delivery,
	delivery_latitude, delivery_longitude, contract_hash,
	expires_at, delivery_confirmed_at, return_accepted, return_rejection_reason,
	created_at, updated_at
`

func scanRental(row interface {
	Scan(dest ...any) error
}, r *model.Rental) error {
	return row.Scan(
		&r.ID,
		&r.ToolID,
		&r.RequesterID,
		&r.Days,
		&r.PricePerDay,
		&r.Total,
		&r.Deposit,
		&r.Status,
		&r.PaymentID,
		&r.PaymentURL,
		&r.RequesterConfirmedDelivery,
		&r.OwnerConfirmedDelivery,
		&r.DeliveryLatitude,
		&r.DeliveryLongitude,
		&r.ContractHash,
		&r.ExpiresAt,
		&r.DeliveryConfirmedAt,
		&r.ReturnAccepted,
		&r.ReturnRejectionReason,
		&r.CreatedAt,
		&r.UpdatedAt,
	)
}

// Create inserta una nueva orden de renta.
func (r *RentalRepository) Create(ctx context.Context, rental *model.Rental) (*model.Rental, error) {
	query := `
		INSERT INTO rentals (tool_id, requester_id, days, price_per_day, total, deposit, status, payment_url, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING ` + rentalSelectColumns

	created := &model.Rental{}
	err := scanRental(r.db.QueryRow(ctx, query,
		rental.ToolID,
		rental.RequesterID,
		rental.Days,
		rental.PricePerDay,
		rental.Total,
		rental.Deposit,
		rental.Status,
		rental.PaymentURL,
		rental.ExpiresAt,
	), created)
	if err != nil {
		return nil, fmt.Errorf("error al crear renta: %w", err)
	}
	return created, nil
}

// FindByID busca una renta por su UUID.
func (r *RentalRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Rental, error) {
	query := `SELECT ` + rentalSelectColumns + ` FROM rentals WHERE id = $1`
	rental := &model.Rental{}
	if err := scanRental(r.db.QueryRow(ctx, query, id), rental); err != nil {
		return nil, fmt.Errorf("renta no encontrada: %w", err)
	}
	return rental, nil
}

// ConfirmPayment actualiza el estado a funds_held y guarda el payment_id de MP.
func (r *RentalRepository) ConfirmPayment(ctx context.Context, id uuid.UUID, paymentID string) (*model.Rental, error) {
	query := `
		UPDATE rentals
		SET status = $1, payment_id = $2
		WHERE id = $3 AND status = 'pending_payment'
		RETURNING ` + rentalSelectColumns

	rental := &model.Rental{}
	err := scanRental(r.db.QueryRow(ctx, query, model.RentalStatusFundsHeld, paymentID, id), rental)
	if err != nil {
		return nil, fmt.Errorf("error al confirmar pago: %w", err)
	}
	return rental, nil
}

// ConfirmDelivery registra la confirmación de entrega de uno de los roles.
// Cuando ambos confirman, cambia el estado a "delivered" y genera el hash del contrato.
func (r *RentalRepository) ConfirmDelivery(
	ctx context.Context,
	id uuid.UUID,
	role string,
	lat, lng float64,
	contractHash string,
) (*model.Rental, error) {
	var query string
	now := time.Now().UTC()

	if role == "requester" {
		query = `
			UPDATE rentals
			SET requester_confirmed_delivery = TRUE,
			    delivery_latitude  = $2,
			    delivery_longitude = $3
			WHERE id = $1
			RETURNING ` + rentalSelectColumns
	} else {
		query = `
			UPDATE rentals
			SET owner_confirmed_delivery = TRUE,
			    delivery_latitude  = $2,
			    delivery_longitude = $3
			WHERE id = $1
			RETURNING ` + rentalSelectColumns
	}

	rental := &model.Rental{}
	if err := scanRental(r.db.QueryRow(ctx, query, id, lat, lng), rental); err != nil {
		return nil, fmt.Errorf("error al confirmar entrega: %w", err)
	}

	// Si ambos confirmaron, actualizar estado a delivered + guardar hash
	if rental.RequesterConfirmedDelivery && rental.OwnerConfirmedDelivery {
		updateQuery := `
			UPDATE rentals
			SET status = $1, contract_hash = $2, delivery_confirmed_at = $3
			WHERE id = $4
			RETURNING ` + rentalSelectColumns

		if err := scanRental(r.db.QueryRow(ctx, updateQuery,
			model.RentalStatusDelivered, contractHash, now, id,
		), rental); err != nil {
			return nil, fmt.Errorf("error al finalizar confirmación de entrega: %w", err)
		}
	}

	return rental, nil
}

// ProcessReturn actualiza la renta cuando el propietario acepta o rechaza la devolución.
func (r *RentalRepository) ProcessReturn(ctx context.Context, id uuid.UUID, accepted bool, reason string) (*model.Rental, error) {
	var newStatus model.RentalStatus
	if accepted {
		newStatus = model.RentalStatusCompleted
	} else {
		newStatus = model.RentalStatusDispute
	}

	query := `
		UPDATE rentals
		SET status = $1, return_accepted = $2, return_rejection_reason = $3
		WHERE id = $4
		RETURNING ` + rentalSelectColumns

	rental := &model.Rental{}
	err := scanRental(r.db.QueryRow(ctx, query, newStatus, accepted, reason, id), rental)
	if err != nil {
		return nil, fmt.Errorf("error al procesar devolución: %w", err)
	}
	return rental, nil
}
