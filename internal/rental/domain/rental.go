package rentaldomain

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

// ServiceCommissionRate es la comisión de servicio de ToolShare: un cargo
// adicional y NO reembolsable sobre el monto de la renta (ingreso de la
// plataforma), distinto del depósito de garantía (que sí se libera si no hay
// disputa).
const ServiceCommissionRate = 0.05

// DepositThreshold: herramientas con estimated_value por debajo de este
// monto no piden depósito de garantía. Por debajo de aquí es prácticamente
// todo herramienta manual/medición (ver catalogo_semilla) donde la fricción
// de pedir depósito le cuesta más a la adopción que lo que protege; arriba
// caen los eléctricos/neumáticos/energía, donde sí hay valor real en riesgo.
const DepositThreshold = 1000.0

// DepositRate es el porcentaje del valor estimado que se retiene como
// depósito de garantía para herramientas por arriba de DepositThreshold.
const DepositRate = 0.10

// Rental representa una renta de herramienta.
type Rental struct {
	ID            uuid.UUID
	ToolID        uuid.UUID
	RequesterID   uuid.UUID
	OwnerID       uuid.UUID
	OwnerName     string
	RequesterName string
	StartDate     time.Time
	EndDate       time.Time
	DailyRate     float64
	TotalAmount   float64
	Status        RentalStatus

	// Método y Pasarela de Pagos
	PaymentMethod    string // "card" o "cash"
	MPPaymentID      string
	PaymentStatus    string
	DeductibleAmount float64 // 10 % del valor estimado de la herramienta (depósito reembolsable)
	CommissionAmount float64 // comisión de servicio de ToolShare (no reembolsable)

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

func (r *Rental) ResolveDispute(action, notes string) error {
	if r.Status != RentalStatusDisputed {
		return errors.New("la renta no está en disputa")
	}
	if action == "capture" {
		r.Status = RentalStatusCompleted
	} else {
		r.Status = RentalStatusCancelled
	}
	if notes != "" {
		r.DisputeReason = "[Dictamen Admin - " + action + "]: " + notes + " (Motivo: " + r.DisputeReason + ")"
	}
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

// CalculateCommission calcula la comisión de servicio de ToolShare sobre el
// monto de la renta. Debe llamarse después de CalculateTotal().
func (r *Rental) CalculateCommission() float64 {
	return r.TotalAmount * ServiceCommissionRate
}
