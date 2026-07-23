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
	userports "github.com/yourusername/tool-inventory-api/internal/user/ports"
)

var (
	ErrToolNotAvailable     = errors.New("la herramienta no está disponible para rentar")
	ErrUnauthorized         = errors.New("no tienes permiso para esta acción")
	ErrPaymentFailed        = errors.New("el pago no pudo procesarse")
	ErrPaymentMethodNotCard = errors.New("solo se acepta pago con tarjeta")
	ErrOwnerMPNotConnected  = errors.New("el propietario no ha vinculado su cuenta de Mercado Pago, no se puede procesar el pago")
)

type rentalService struct {
	rentalRepo      rentalports.RentalRepository
	toolRepo        toolports.ToolRepository
	userRepo        userports.UserRepository
	paymentProvider sharedports.PaymentProvider // nil en modo desarrollo
}

func NewRentalService(
	rentalRepo rentalports.RentalRepository,
	toolRepo toolports.ToolRepository,
	userRepo userports.UserRepository,
	paymentProvider sharedports.PaymentProvider,
) rentalports.RentalService {
	return &rentalService{
		rentalRepo:      rentalRepo,
		toolRepo:        toolRepo,
		userRepo:        userRepo,
		paymentProvider: paymentProvider,
	}
}

// sellerAccessToken busca la cuenta de Mercado Pago vinculada por el
// propietario (ownerID) y devuelve un access_token utilizable para crear/
// gestionar pagos en su nombre (split de marketplace). Si el token guardado
// ya expiró, lo renueva primero. Devuelve error si el propietario no ha
// vinculado ninguna cuenta.
func (s *rentalService) sellerAccessToken(ctx context.Context, ownerID uuid.UUID) (string, error) {
	return resolveSellerAccessToken(ctx, s.userRepo, s.paymentProvider, ownerID)
}

// resolveSellerAccessToken es la lógica compartida entre rentalService y
// adminService para obtener (y renovar si hace falta) el access_token de
// Mercado Pago del propietario, usado para crear/gestionar pagos con split.
func resolveSellerAccessToken(ctx context.Context, userRepo userports.UserRepository, paymentProvider sharedports.PaymentProvider, ownerID uuid.UUID) (string, error) {
	if paymentProvider.IsMock() {
		return "", nil
	}

	account, err := userRepo.GetMPSellerAccount(ctx, ownerID)
	if err != nil {
		return "", err
	}
	if !account.Connected() {
		return "", ErrOwnerMPNotConnected
	}
	if time.Now().Before(account.ExpiresAt) {
		return account.AccessToken, nil
	}

	accessToken, refreshToken, expiresAt, err := paymentProvider.RefreshSellerToken(ctx, account.RefreshToken)
	if err != nil {
		return "", fmt.Errorf("renovar token de Mercado Pago del propietario: %w", err)
	}
	if err := userRepo.SetMPSellerAccount(ctx, ownerID, account.SellerUserID, accessToken, refreshToken, expiresAt); err != nil {
		log.Printf("WARN: no se pudo guardar el token renovado del propietario %s: %v", ownerID, err)
	}
	return accessToken, nil
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

	method := inp.PaymentMethod
	if method == "" {
		method = "card"
	}
	if method != "card" && method != "cash" {
		return nil, ErrPaymentMethodNotCard
	}

	rental := &rentaldomain.Rental{
		ToolID:        inp.ToolID,
		RequesterID:   inp.RequesterID,
		OwnerID:       tool.OwnerID,
		StartDate:     inp.StartDate,
		EndDate:       inp.EndDate,
		DailyRate:     tool.DailyRate,
		PaymentMethod: method,
		Status:        rentaldomain.RentalStatusPending,
	}
	rental.TotalAmount = rental.CalculateTotal()
	rental.CommissionAmount = rental.CalculateCommission()

	// Por debajo de DepositThreshold no se pide depósito: la fricción de
	// pedirlo en herramienta manual barata cuesta más en adopción que lo que
	// protege. Arriba del umbral, el depósito protege según lo que vale la
	// herramienta, no según cuánto cuesta rentarla un día — así que en rentas
	// cortas se tope a 2x el total de la renta para que no se sienta
	// absurdamente desproporcionado (en rentas largas el total ya supera el
	// 10% del valor por si solo, así que el tope no aplica ahi).
	var deductible float64
	if tool.EstimatedValue >= rentaldomain.DepositThreshold {
		deductible = tool.EstimatedValue * rentaldomain.DepositRate
		if cap := rental.TotalAmount * 2; deductible > cap {
			deductible = cap
		}
	}
	rental.DeductibleAmount = deductible

	// Pre-autorizar pago si se proporcionó token de tarjeta
	var sellerToken string
	if inp.CardToken != "" && s.paymentProvider != nil {
		var err error
		sellerToken, err = s.sellerAccessToken(ctx, tool.OwnerID)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrPaymentFailed, err)
		}

		totalToFreeze := rental.TotalAmount + rental.CommissionAmount + deductible
		paymentID, err := s.paymentProvider.Authorize(ctx, sharedports.AuthorizePaymentInput{
			Amount:            totalToFreeze,
			CardToken:         inp.CardToken,
			Description:       fmt.Sprintf("Renta de herramienta: %.0f días", rental.TotalAmount/rental.DailyRate),
			PayerEmail:        inp.PayerEmail,
			Metadata:          map[string]string{"tool_id": tool.ID.String()},
			SellerAccessToken: sellerToken,
			ApplicationFee:    rental.CommissionAmount,
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
			_ = s.paymentProvider.Cancel(ctx, rental.MPPaymentID, sellerToken)
		}
		return nil, err
	}

	// En efectivo el bloqueo es inmediato: no hay pasarela de por medio, la
	// reserva en sí ya compromete la herramienta. En tarjeta, el pago real
	// ocurre después en el checkout de Mercado Pago (Checkout Pro) y se
	// confirma vía webhook — recién ahí (UpdatePaymentStatus, status
	// "approved") se bloquea la herramienta. Antes de eso solo es una
	// solicitud pendiente de pago; no debe verse como "rentada" sin que se
	// haya cobrado nada todavía.
	if method == "cash" {
		if err := s.toolRepo.SetAvailability(ctx, tool.ID, false); err != nil {
			return nil, err
		}
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
			sellerToken, tokenErr := s.sellerAccessToken(ctx, updated.OwnerID)
			if tokenErr != nil {
				log.Printf("WARN: no se pudo obtener token del propietario para capturar pago %s: %v", updated.MPPaymentID, tokenErr)
			} else if err := s.paymentProvider.Capture(ctx, updated.MPPaymentID, amountToCapture, sellerToken); err != nil {
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
		sellerToken, tokenErr := s.sellerAccessToken(ctx, updated.OwnerID)
		if tokenErr != nil {
			log.Printf("WARN: no se pudo obtener token del propietario para capturar depósito %s: %v", updated.MPPaymentID, tokenErr)
		} else if err := s.paymentProvider.Capture(ctx, updated.MPPaymentID, updated.DeductibleAmount, sellerToken); err != nil {
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
		sellerToken, tokenErr := s.sellerAccessToken(ctx, updated.OwnerID)
		if tokenErr != nil {
			log.Printf("WARN: no se pudo obtener token del propietario para cancelar pago %s: %v", updated.MPPaymentID, tokenErr)
		} else if err := s.paymentProvider.Cancel(ctx, updated.MPPaymentID, sellerToken); err != nil {
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

	sellerToken, err := s.sellerAccessToken(ctx, rental.OwnerID)
	if err != nil {
		return sharedports.CreatePreferenceOutput{}, fmt.Errorf("%w: %v", ErrPaymentFailed, err)
	}

	totalToFreeze := rental.TotalAmount + rental.CommissionAmount + rental.DeductibleAmount

	notificationURL := os.Getenv("MP_NOTIFICATION_URL")
	backURL := os.Getenv("MP_BACK_URL")
	if backURL == "" {
		// auto_return de MP exige una URL absoluta http(s); "toolshare://" no
		// es válida y deja el botón "Pagar" inerte. El WebView de Flutter
		// intercepta esta ruta antes de que la navegación se complete.
		backURL = "https://toolshare-api.up.railway.app/payment"
	}

	out, err := s.paymentProvider.CreatePreference(ctx, sharedports.CreatePreferenceInput{
		Title:             fmt.Sprintf("Renta de herramienta (%s)", rentalID.String()[:8]),
		TotalAmount:       totalToFreeze,
		PayerEmail:        payerEmail,
		ExternalRef:       rental.ID.String(),
		NotificationURL:   notificationURL,
		BackURLSuccess:    backURL + "/success",
		BackURLFailure:    backURL + "/failure",
		BackURLPending:    backURL + "/pending",
		SellerAccessToken: sellerToken,
		ApplicationFee:    rental.CommissionAmount,
	})
	if err != nil {
		return sharedports.CreatePreferenceOutput{}, fmt.Errorf("%w: %v", ErrPaymentFailed, err)
	}

	return out, nil
}

func (s *rentalService) UpdatePaymentStatus(ctx context.Context, rentalID uuid.UUID, paymentID, status, paymentTypeID string) error {
	rental, err := s.rentalRepo.FindByID(ctx, rentalID)
	if err != nil {
		return err
	}

	// Checkout Pro no deja excluir "account_money" (MP responde 400 si se
	// intenta), así que un comprador puede aprobar el pago con el saldo de
	// su cuenta MP en vez de una tarjeta real aunque la app solo ofrezca
	// "Tarjeta". Esa combinación no es válida para este flujo: se reembolsa
	// de inmediato y se trata como pago rechazado, para que la herramienta
	// nunca quede marcada como rentada por dinero que no vino de una
	// tarjeta real.
	if status == "approved" && rental.PaymentMethod == "card" &&
		paymentTypeID != "" && paymentTypeID != "credit_card" && paymentTypeID != "debit_card" {
		sellerToken, tokenErr := s.sellerAccessToken(ctx, rental.OwnerID)
		if tokenErr != nil {
			log.Printf("WARN: no se pudo obtener token del propietario para reembolsar pago %s (payment_type_id=%s): %v", paymentID, paymentTypeID, tokenErr)
		} else if err := s.paymentProvider.Refund(ctx, paymentID, 0, sellerToken); err != nil {
			log.Printf("WARN: no se pudo reembolsar pago %s con payment_type_id=%s: %v", paymentID, paymentTypeID, err)
		} else {
			log.Printf("Pago %s reembolsado: payment_type_id=%s no es tarjeta (renta %s)", paymentID, paymentTypeID, rentalID)
		}
		status = "rejected"
	}

	rental.MPPaymentID = paymentID
	rental.PaymentStatus = status

	switch status {
	case "approved":
		// Recién aquí se confirma que el pago con tarjeta se cobró de
		// verdad: apenas ahora se bloquea la herramienta como rentada (ver
		// nota en Create).
		if rental.PaymentMethod == "card" {
			_ = s.toolRepo.SetAvailability(ctx, rental.ToolID, false)
		}
	case "rejected", "cancelled":
		// El pago no se completó: si la solicitud seguía pendiente de pago,
		// se cancela para no dejarla colgada apareciéndole al solicitante
		// como una reserva viva que nunca se cobró.
		if rental.Status == rentaldomain.RentalStatusPending {
			rental.Status = rentaldomain.RentalStatusCancelled
		}
	}

	_, err = s.rentalRepo.Update(ctx, rental)
	if err == nil {
		pubsub.Publish(rental.ID)
	}
	return err
}

// ConfirmPayment es el respaldo del webhook: lo llama el frontend en cuanto
// Mercado Pago lo redirige a la URL de éxito del checkout. Nunca confía en
// que "ya pasó por la pasarela" signifique que se pagó — vuelve a consultar
// el pago directo con la API de MP (misma fuente de verdad que usa el
// webhook) antes de tocar el estado de la renta o la herramienta.
func (s *rentalService) ConfirmPayment(ctx context.Context, rentalID uuid.UUID, requesterID uuid.UUID, paymentID string) (*rentaldomain.Rental, error) {
	rental, err := s.rentalRepo.FindByID(ctx, rentalID)
	if err != nil {
		return nil, err
	}
	if rental.RequesterID != requesterID {
		return nil, ErrUnauthorized
	}
	if s.paymentProvider == nil {
		return nil, fmt.Errorf("%w: pasarela de pagos no configurada", ErrPaymentFailed)
	}

	info, err := s.paymentProvider.GetPaymentInfo(ctx, paymentID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPaymentFailed, err)
	}
	// El payment_id lo manda el cliente (viene de la URL de retorno de MP);
	// se valida contra la renta a la que dice pertenecer antes de aceptarlo,
	// para que no se pueda confirmar una renta ajena con el ID de otro pago.
	if info.ExternalRef != rentalID.String() {
		return nil, fmt.Errorf("%w: el pago no corresponde a esta renta", ErrPaymentFailed)
	}

	if err := s.UpdatePaymentStatus(ctx, rentalID, info.ID, info.Status, info.PaymentTypeID); err != nil {
		return nil, err
	}
	return s.rentalRepo.FindByID(ctx, rentalID)
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
