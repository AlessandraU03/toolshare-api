package rentalservice

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	rentaldomain "github.com/yourusername/tool-inventory-api/internal/rental/domain"
	rentalports "github.com/yourusername/tool-inventory-api/internal/rental/ports"
	sharedports "github.com/yourusername/tool-inventory-api/internal/shared/ports"
	toolports "github.com/yourusername/tool-inventory-api/internal/tool/ports"
)

var (
	ErrToolNotAvailable = errors.New("la herramienta no está disponible para rentar")
	ErrUnauthorized     = errors.New("no tienes permiso para esta acción")
	ErrPaymentFailed    = errors.New("el pago no pudo procesarse")
)

type rentalService struct {
	rentalRepo      rentalports.RentalRepository
	toolRepo        toolports.ToolRepository
	paymentProvider sharedports.PaymentProvider // nil en modo desarrollo
}

func NewRentalService(
	rentalRepo rentalports.RentalRepository,
	toolRepo toolports.ToolRepository,
	paymentProvider sharedports.PaymentProvider,
) rentalports.RentalService {
	return &rentalService{
		rentalRepo:      rentalRepo,
		toolRepo:        toolRepo,
		paymentProvider: paymentProvider,
	}
}

func (s *rentalService) Create(ctx context.Context, inp rentalports.CreateRentalInput) (*rentaldomain.Rental, error) {
	tool, err := s.toolRepo.FindByID(ctx, inp.ToolID)
	if err != nil {
		return nil, err
	}
	if !tool.IsAvailable {
		return nil, ErrToolNotAvailable
	}

	deductible := tool.EstimatedValue * 0.10

	rental := &rentaldomain.Rental{
		ToolID:           inp.ToolID,
		RequesterID:      inp.RequesterID,
		OwnerID:          tool.OwnerID,
		StartDate:        inp.StartDate,
		EndDate:          inp.EndDate,
		DailyRate:        tool.DailyRate,
		DeductibleAmount: deductible,
		Status:           rentaldomain.RentalStatusPending,
	}
	rental.TotalAmount = rental.CalculateTotal()

	// Pre-autorizar pago si se proporcionó token de tarjeta
	if inp.CardToken != "" && s.paymentProvider != nil {
		totalToFreeze := rental.TotalAmount + deductible
		paymentID, err := s.paymentProvider.Authorize(ctx, sharedports.AuthorizePaymentInput{
			Amount:      totalToFreeze,
			CardToken:   inp.CardToken,
			Description: fmt.Sprintf("Renta de herramienta: %.0f días", rental.TotalAmount/rental.DailyRate),
			PayerEmail:  inp.PayerEmail,
			Metadata:    map[string]string{"tool_id": tool.ID.String()},
		})
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrPaymentFailed, err)
		}
		rental.MPPaymentID = paymentID
		rental.PaymentStatus = "authorized"
	}

	created, err := s.rentalRepo.Create(ctx, rental)
	if err != nil {
		// Si falló la BD pero el pago ya fue autorizado, cancelarlo
		if rental.MPPaymentID != "" {
			_ = s.paymentProvider.Cancel(ctx, rental.MPPaymentID)
		}
		return nil, err
	}

	if err := s.toolRepo.SetAvailability(ctx, tool.ID, false); err != nil {
		return nil, err
	}

	return created, nil
}

func (s *rentalService) GetByID(ctx context.Context, id uuid.UUID) (*rentaldomain.Rental, error) {
	return s.rentalRepo.FindByID(ctx, id)
}

func (s *rentalService) ListByUser(ctx context.Context, userID uuid.UUID) ([]*rentaldomain.Rental, error) {
	return s.rentalRepo.FindByUser(ctx, userID)
}

// ConfirmDelivery implementa el apretón de manos digital en la entrega.
// Cuando ambas partes confirman, genera el contrato SHA-256 y activa la renta.
func (s *rentalService) ConfirmDelivery(ctx context.Context, rentalID uuid.UUID, userID uuid.UUID, lat, lng float64) (*rentaldomain.Rental, error) {
	rental, err := s.rentalRepo.FindByID(ctx, rentalID)
	if err != nil {
		return nil, err
	}

	switch userID {
	case rental.OwnerID:
		if err := rental.ConfirmDeliveryByOwner(); err != nil {
			return nil, err
		}
	case rental.RequesterID:
		if err := rental.ConfirmDeliveryByRequester(); err != nil {
			return nil, err
		}
	default:
		return nil, ErrUnauthorized
	}

	// Ambas partes confirmaron → activar renta y generar contrato
	if rental.BothConfirmedDelivery() {
		rental.Activate()

		now := time.Now().UTC()
		rental.DeliveryAt = &now
		rental.DeliveryLat = lat
		rental.DeliveryLng = lng

		// Hash SHA-256 como contrato inmutable
		raw := fmt.Sprintf("%s|%s|%s|%s|%s|%.6f|%.6f",
			rental.ID, rental.ToolID, rental.OwnerID, rental.RequesterID,
			now.Format(time.RFC3339), lat, lng)
		hash := sha256.Sum256([]byte(raw))
		rental.ContractHash = fmt.Sprintf("%x", hash)
	}

	return s.rentalRepo.Update(ctx, rental)
}

// ConfirmReturn implementa el apretón de manos en la devolución.
// Cuando ambas partes confirman, cobra la renta en MP y libera el depósito.
func (s *rentalService) ConfirmReturn(ctx context.Context, rentalID uuid.UUID, userID uuid.UUID) (*rentaldomain.Rental, error) {
	rental, err := s.rentalRepo.FindByID(ctx, rentalID)
	if err != nil {
		return nil, err
	}

	switch userID {
	case rental.OwnerID:
		if err := rental.ConfirmReturnByOwner(); err != nil {
			return nil, err
		}
	case rental.RequesterID:
		if err := rental.ConfirmReturnByRequester(); err != nil {
			return nil, err
		}
	default:
		return nil, ErrUnauthorized
	}

	updated, err := s.rentalRepo.Update(ctx, rental)
	if err != nil {
		return nil, err
	}

	if updated.Status == rentaldomain.RentalStatusCompleted {
		_ = s.toolRepo.SetAvailability(ctx, updated.ToolID, true)

		// Captura parcial: cobra solo la renta (el depósito se libera automáticamente)
		if s.paymentProvider != nil && updated.MPPaymentID != "" {
			if err := s.paymentProvider.Capture(ctx, updated.MPPaymentID, updated.TotalAmount); err != nil {
				log.Printf("WARN: no se pudo capturar pago %s: %v", updated.MPPaymentID, err)
			} else {
				updated.PaymentStatus = "captured"
				updated, _ = s.rentalRepo.Update(ctx, updated)
			}
		}
	}

	return updated, nil
}

// Dispute registra que el propietario detectó daño en la herramienta devuelta.
// Captura el depósito como penalización y bloquea el pago pendiente de arbitraje.
func (s *rentalService) Dispute(ctx context.Context, rentalID uuid.UUID, ownerID uuid.UUID, reason string) (*rentaldomain.Rental, error) {
	rental, err := s.rentalRepo.FindByID(ctx, rentalID)
	if err != nil {
		return nil, err
	}

	if rental.OwnerID != ownerID {
		return nil, ErrUnauthorized
	}

	if err := rental.Dispute(reason); err != nil {
		return nil, err
	}

	updated, err := s.rentalRepo.Update(ctx, rental)
	if err != nil {
		return nil, err
	}

	// Capturar el depósito como penalización al solicitante
	if s.paymentProvider != nil && updated.MPPaymentID != "" {
		if err := s.paymentProvider.Capture(ctx, updated.MPPaymentID, updated.DeductibleAmount); err != nil {
			log.Printf("WARN: no se pudo capturar depósito en disputa %s: %v", updated.MPPaymentID, err)
		} else {
			updated.PaymentStatus = "captured"
			updated, _ = s.rentalRepo.Update(ctx, updated)
		}
	}

	return updated, nil
}

func (s *rentalService) Cancel(ctx context.Context, rentalID uuid.UUID, userID uuid.UUID) (*rentaldomain.Rental, error) {
	rental, err := s.rentalRepo.FindByID(ctx, rentalID)
	if err != nil {
		return nil, err
	}

	if rental.OwnerID != userID && rental.RequesterID != userID {
		return nil, ErrUnauthorized
	}

	if err := rental.Cancel(); err != nil {
		return nil, err
	}

	updated, err := s.rentalRepo.Update(ctx, rental)
	if err != nil {
		return nil, err
	}

	_ = s.toolRepo.SetAvailability(ctx, updated.ToolID, true)

	// Cancelar la pre-autorización en MP (devuelve fondos al solicitante)
	if s.paymentProvider != nil && updated.MPPaymentID != "" {
		if err := s.paymentProvider.Cancel(ctx, updated.MPPaymentID); err != nil {
			log.Printf("WARN: no se pudo cancelar pago %s: %v", updated.MPPaymentID, err)
		}
	}

	return updated, nil
}
