package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type RentalStatus string

const (
	RentalStatusPending   RentalStatus = "pending"
	RentalStatusActive    RentalStatus = "active"
	RentalStatusCompleted RentalStatus = "completed"
	RentalStatusCancelled RentalStatus = "cancelled"
	RentalStatusDisputed  RentalStatus = "disputed"
)

// Rental representa una renta de herramienta.
//
// Flujo de estados:
//  1. Requester crea la renta → pending (fondos congelados en MP)
//  2. Ambos confirman entrega → active (contrato SHA-256 generado)
//  3a. Ambos confirman devolución → completed (MP captura renta, devuelve depósito)
//  3b. Propietario disputa → disputed (MP captura el depósito)
type Rental struct {
	ID          uuid.UUID
	ToolID      uuid.UUID
	RequesterID uuid.UUID
	OwnerID     uuid.UUID
	StartDate   time.Time
	EndDate     time.Time
	DailyRate   float64
	TotalAmount float64
	Status      RentalStatus

	// Mercado Pago
	MPPaymentID      string
	PaymentStatus    string
	DeductibleAmount float64 // 10 % del valor estimado de la herramienta

	// Paso 2: apretón de manos en la entrega
	OwnerConfirmedDelivery     bool
	RequesterConfirmedDelivery bool

	// Contrato digital generado cuando ambos confirman la entrega
	ContractHash string
	DeliveryLat  float64
	DeliveryLng  float64
	DeliveryAt   *time.Time

	// Paso 3: apretón de manos en la devolución
	RequesterConfirmedReturn bool
	OwnerConfirmedReturn     bool

	// Disputa
	DisputeReason string

	CreatedAt time.Time
	UpdatedAt time.Time
}

var (
	ErrRentalNotPending = errors.New("la renta no está en estado pendiente")
	ErrRentalNotActive  = errors.New("la renta no está activa")
	ErrAlreadyConfirmed = errors.New("ya confirmaste esta acción")
	ErrCannotCancelDone = errors.New("no se puede cancelar una renta completada")
)

func (r *Rental) ConfirmDeliveryByOwner() error {
	if r.Status != RentalStatusPending {
		return ErrRentalNotPending
	}
	if r.OwnerConfirmedDelivery {
		return ErrAlreadyConfirmed
	}
	r.OwnerConfirmedDelivery = true
	return nil
}

func (r *Rental) ConfirmDeliveryByRequester() error {
	if r.Status != RentalStatusPending {
		return ErrRentalNotPending
	}
	if r.RequesterConfirmedDelivery {
		return ErrAlreadyConfirmed
	}
	r.RequesterConfirmedDelivery = true
	return nil
}

// BothConfirmedDelivery reporta si ambas partes han confirmado la entrega.
func (r *Rental) BothConfirmedDelivery() bool {
	return r.OwnerConfirmedDelivery && r.RequesterConfirmedDelivery
}

func (r *Rental) Activate() {
	r.Status = RentalStatusActive
}

func (r *Rental) ConfirmReturnByRequester() error {
	if r.Status != RentalStatusActive {
		return ErrRentalNotActive
	}
	if r.RequesterConfirmedReturn {
		return ErrAlreadyConfirmed
	}
	r.RequesterConfirmedReturn = true
	r.checkComplete()
	return nil
}

func (r *Rental) ConfirmReturnByOwner() error {
	if r.Status != RentalStatusActive {
		return ErrRentalNotActive
	}
	if r.OwnerConfirmedReturn {
		return ErrAlreadyConfirmed
	}
	r.OwnerConfirmedReturn = true
	r.checkComplete()
	return nil
}

func (r *Rental) checkComplete() {
	if r.RequesterConfirmedReturn && r.OwnerConfirmedReturn {
		r.Status = RentalStatusCompleted
	}
}

// Dispute registra una disputa por daño. Solo el propietario puede llamarlo
// y la renta debe estar activa.
func (r *Rental) Dispute(reason string) error {
	if r.Status != RentalStatusActive {
		return ErrRentalNotActive
	}
	r.Status = RentalStatusDisputed
	r.DisputeReason = reason
	return nil
}

func (r *Rental) Cancel() error {
	if r.Status == RentalStatusCompleted {
		return ErrCannotCancelDone
	}
	r.Status = RentalStatusCancelled
	return nil
}

func (r *Rental) CalculateTotal() float64 {
	days := r.EndDate.Sub(r.StartDate).Hours() / 24
	if days < 1 {
		days = 1
	}
	return r.DailyRate * days
}
