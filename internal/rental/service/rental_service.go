package rentalservice

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	rentaldomain "github.com/yourusername/tool-inventory-api/internal/rental/domain"
	rentalports "github.com/yourusername/tool-inventory-api/internal/rental/ports"
	sharedports "github.com/yourusername/tool-inventory-api/internal/shared/ports"
	"github.com/yourusername/tool-inventory-api/internal/shared/pubsub"
	toolports "github.com/yourusername/tool-inventory-api/internal/tool/ports"
)

var (
	ErrToolNotAvailable     = errors.New("la herramienta no está disponible para rentar")
	ErrUnauthorized         = errors.New("no tienes permiso para esta acción")
	ErrPaymentFailed        = errors.New("el pago no pudo procesarse")
	ErrPaymentMethodNotCard = errors.New("solo se acepta pago con tarjeta")
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

	// Device Fingerprinting: Registrar huella del solicitante
	if inp.IPAddress != "" && inp.DeviceID != "" {
		_ = s.rentalRepo.LogFingerprint(ctx, inp.RequesterID, inp.IPAddress, inp.DeviceID)
	}

	// Detección de Colusión (Mismo dispositivo/red)
	collusion, err := s.rentalRepo.CheckCollusion(ctx, tool.OwnerID, inp.RequesterID)
	if err == nil && collusion {
		return nil, errors.New("riesgo_colusion: se detectó coincidencia de dispositivo o red local entre el propietario y el arrendatario")
	}

	deductible := tool.EstimatedValue * 0.10

	method := inp.PaymentMethod
	if method == "" {
		method = "card"
	}
	if method != "card" {
		return nil, ErrPaymentMethodNotCard
	}

	rental := &rentaldomain.Rental{
		ToolID:           inp.ToolID,
		RequesterID:      inp.RequesterID,
		OwnerID:          tool.OwnerID,
		StartDate:        inp.StartDate,
		EndDate:          inp.EndDate,
		DailyRate:        tool.DailyRate,
		DeductibleAmount: deductible,
		PaymentMethod:    method,
		Status:           rentaldomain.RentalStatusPending,
	}
	rental.TotalAmount = rental.CalculateTotal()
	rental.CommissionAmount = rental.CalculateCommission()

	// Pre-autorizar pago si se proporcionó token de tarjeta
	if inp.CardToken != "" && s.paymentProvider != nil {
		totalToFreeze := rental.TotalAmount + rental.CommissionAmount + deductible
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

	updated, err := s.rentalRepo.Update(ctx, rental)
	if err == nil {
		pubsub.Publish(rental.ID)
	}
	return updated, err
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
	pubsub.Publish(rental.ID)

	if updated.Status == rentaldomain.RentalStatusCompleted {
		_ = s.toolRepo.SetAvailability(ctx, updated.ToolID, true)

		// Captura parcial: cobra la renta + comisión de servicio de ToolShare
		// (el depósito de garantía se libera automáticamente al no haber disputa)
		if s.paymentProvider != nil && updated.MPPaymentID != "" {
			amountToCapture := updated.TotalAmount + updated.CommissionAmount
			if err := s.paymentProvider.Capture(ctx, updated.MPPaymentID, amountToCapture); err != nil {
				log.Printf("WARN: no se pudo capturar pago %s: %v", updated.MPPaymentID, err)
			} else {
				updated.PaymentStatus = "captured"
				updated, _ = s.rentalRepo.Update(ctx, updated)
				pubsub.Publish(updated.ID)
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
	pubsub.Publish(rental.ID)

	// Capturar el depósito como penalización al solicitante
	if s.paymentProvider != nil && updated.MPPaymentID != "" {
		if err := s.paymentProvider.Capture(ctx, updated.MPPaymentID, updated.DeductibleAmount); err != nil {
			log.Printf("WARN: no se pudo capturar depósito en disputa %s: %v", updated.MPPaymentID, err)
		} else {
			updated.PaymentStatus = "captured"
			updated, _ = s.rentalRepo.Update(ctx, updated)
			pubsub.Publish(updated.ID)
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
	pubsub.Publish(rental.ID)

	_ = s.toolRepo.SetAvailability(ctx, updated.ToolID, true)

	// Cancelar la pre-autorización en MP (devuelve fondos al solicitante)
	if s.paymentProvider != nil && updated.MPPaymentID != "" {
		if err := s.paymentProvider.Cancel(ctx, updated.MPPaymentID); err != nil {
			log.Printf("WARN: no se pudo cancelar pago %s: %v", updated.MPPaymentID, err)
		}
	}

	return updated, nil
}

func (s *rentalService) CreatePreference(ctx context.Context, rentalID uuid.UUID, requesterID uuid.UUID, payerEmail string) (sharedports.CreatePreferenceOutput, error) {
	if s.paymentProvider == nil {
		return sharedports.CreatePreferenceOutput{}, fmt.Errorf("%w: pasarela de pagos no configurada", ErrPaymentFailed)
	}

	rental, err := s.rentalRepo.FindByID(ctx, rentalID)
	if err != nil {
		return sharedports.CreatePreferenceOutput{}, err
	}
	if rental.RequesterID != requesterID {
		return sharedports.CreatePreferenceOutput{}, ErrUnauthorized
	}

	totalToFreeze := rental.TotalAmount + rental.CommissionAmount + rental.DeductibleAmount

	notificationURL := os.Getenv("MP_NOTIFICATION_URL")
	backURL := os.Getenv("MP_BACK_URL")
	if backURL == "" {
		backURL = "toolshare://payment"
	}

	out, err := s.paymentProvider.CreatePreference(ctx, sharedports.CreatePreferenceInput{
		Title:           fmt.Sprintf("Renta de herramienta (%s)", rentalID.String()[:8]),
		TotalAmount:     totalToFreeze,
		PayerEmail:      payerEmail,
		ExternalRef:     rental.ID.String(),
		NotificationURL: notificationURL,
		BackURLSuccess:  backURL + "/success",
		BackURLFailure:  backURL + "/failure",
		BackURLPending:  backURL + "/pending",
	})
	if err != nil {
		return sharedports.CreatePreferenceOutput{}, fmt.Errorf("%w: %v", ErrPaymentFailed, err)
	}

	return out, nil
}

func (s *rentalService) UpdatePaymentStatus(ctx context.Context, rentalID uuid.UUID, paymentID, status string) error {
	rental, err := s.rentalRepo.FindByID(ctx, rentalID)
	if err != nil {
		return err
	}
	rental.MPPaymentID = paymentID
	rental.PaymentStatus = status
	_, err = s.rentalRepo.Update(ctx, rental)
	if err == nil {
		pubsub.Publish(rental.ID)
	}
	return err
}

func (s *rentalService) GetMessages(ctx context.Context, rentalID uuid.UUID, userID uuid.UUID) ([]*rentaldomain.Message, error) {
	rental, err := s.rentalRepo.FindByID(ctx, rentalID)
	if err != nil {
		return nil, err
	}
	if rental.OwnerID != userID && rental.RequesterID != userID {
		return nil, ErrUnauthorized
	}
	return s.rentalRepo.GetMessages(ctx, rentalID)
}

func (s *rentalService) SendMessage(ctx context.Context, inp rentalports.SendMessageInput) (*rentaldomain.Message, error) {
	rental, err := s.rentalRepo.FindByID(ctx, inp.RentalID)
	if err != nil {
		return nil, err
	}
	if rental.OwnerID != inp.SenderID && rental.RequesterID != inp.SenderID {
		return nil, ErrUnauthorized
	}
	if strings.TrimSpace(inp.Message) == "" {
		return nil, errors.New("el mensaje no puede estar vacío")
	}
	return s.rentalRepo.CreateMessage(ctx, &rentaldomain.Message{
		RentalID: inp.RentalID,
		SenderID: inp.SenderID,
		Message:  strings.TrimSpace(inp.Message),
	})
}

func (s *rentalService) VerifyContract(ctx context.Context, rentalID uuid.UUID) (bool, string, string, error) {
	rental, err := s.rentalRepo.FindByID(ctx, rentalID)
	if err != nil {
		return false, "", "", err
	}

	if rental.ContractHash == "" {
		return false, "", "", errors.New("esta renta aun no cuenta con un contrato firmado")
	}

	if rental.DeliveryAt == nil {
		return false, "", "", errors.New("fecha de entrega nula en contrato activo")
	}

	// Re-calcular hash usando los datos crudos originales en formato UTC inmutable
	raw := fmt.Sprintf("%s|%s|%s|%s|%s|%.6f|%.6f",
		rental.ID, rental.ToolID, rental.OwnerID, rental.RequesterID,
		rental.DeliveryAt.UTC().Format(time.RFC3339), rental.DeliveryLat, rental.DeliveryLng)
	hash := sha256.Sum256([]byte(raw))
	recalculatedHash := fmt.Sprintf("%x", hash)

	isValid := recalculatedHash == rental.ContractHash
	return isValid, rental.ContractHash, recalculatedHash, nil
}

func (s *rentalService) LogUserFingerprint(ctx context.Context, userID uuid.UUID, ipAddress, deviceID string) error {
	if ipAddress == "" || deviceID == "" {
		return nil
	}
	return s.rentalRepo.LogFingerprint(ctx, userID, ipAddress, deviceID)
}
