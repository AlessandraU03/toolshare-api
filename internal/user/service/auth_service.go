package userservice

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	apperrors "github.com/yourusername/tool-inventory-api/internal/shared/errors"
	sharedports "github.com/yourusername/tool-inventory-api/internal/shared/ports"
	userdomain "github.com/yourusername/tool-inventory-api/internal/user/domain"
	userports "github.com/yourusername/tool-inventory-api/internal/user/ports"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrEmailAlreadyExists = errors.New("el email ya está registrado")
	ErrInvalidCredentials = errors.New("credenciales inválidas")
	ErrPaymentFailed      = errors.New("no se pudo crear la preferencia de pago")
	ErrPaymentNotApproved = errors.New("el pago aún no está aprobado")
	ErrPaymentRefMismatch = errors.New("el pago no corresponde a este usuario")
)

// SubscriptionExternalRefPrefix marca las external_reference de MP que corresponden
// a una suscripción Pro (en vez de una renta), para distinguirlas en el webhook.
const SubscriptionExternalRefPrefix = "sub:"

// ProMonthlyPrice es el precio mensual del plan Pro en MXN.
const ProMonthlyPrice = 69.0

type authService struct {
	userRepo        userports.UserRepository
	tokenProvider   sharedports.TokenProvider
	paymentProvider sharedports.PaymentProvider
}

func NewAuthService(userRepo userports.UserRepository, tokenProvider sharedports.TokenProvider, paymentProvider sharedports.PaymentProvider) userports.AuthService {
	return &authService{userRepo: userRepo, tokenProvider: tokenProvider, paymentProvider: paymentProvider}
}

func (s *authService) Register(ctx context.Context, inp userports.RegisterInput) (*userports.AuthOutput, error) {
	email := strings.ToLower(strings.TrimSpace(inp.Email))

	_, err := s.userRepo.FindByEmail(ctx, email)
	if err == nil {
		return nil, ErrEmailAlreadyExists
	}
	if !errors.Is(err, apperrors.ErrNotFound) {
		return nil, err
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(inp.Password), 12)
	if err != nil {
		return nil, err
	}

	user := &userdomain.User{
		Name:     inp.Name,
		Email:    email,
		Password: string(hashed),
		Role:     inp.Role,
		Phone:    inp.Phone,
		INE:      inp.INE,
	}

	created, err := s.userRepo.Create(ctx, user)
	if err != nil {
		return nil, err
	}

	token, err := s.tokenProvider.Generate(created.ID, created.Role)
	if err != nil {
		return nil, err
	}

	return &userports.AuthOutput{Token: token, User: created}, nil
}

func (s *authService) Login(ctx context.Context, email, password string) (*userports.AuthOutput, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	token, err := s.tokenProvider.Generate(user.ID, user.Role)
	if err != nil {
		return nil, err
	}

	return &userports.AuthOutput{Token: token, User: user}, nil
}

func (s *authService) CreateSubscriptionPreference(ctx context.Context, userID uuid.UUID) (sharedports.CreatePreferenceOutput, error) {
	if s.paymentProvider == nil {
		return sharedports.CreatePreferenceOutput{}, fmt.Errorf("%w: pasarela de pagos no configurada", ErrPaymentFailed)
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return sharedports.CreatePreferenceOutput{}, err
	}

	notificationURL := os.Getenv("MP_NOTIFICATION_URL")
	backURL := os.Getenv("MP_BACK_URL")
	if backURL == "" {
		backURL = "toolshare://payment"
	}

	out, err := s.paymentProvider.CreatePreference(ctx, sharedports.CreatePreferenceInput{
		Title:           "Suscripción ToolShare Pro (mensual)",
		TotalAmount:     ProMonthlyPrice,
		PayerEmail:      user.Email,
		ExternalRef:     SubscriptionExternalRefPrefix + userID.String(),
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

func (s *authService) GetProfile(ctx context.Context, userID uuid.UUID) (*userdomain.User, error) {
	return s.userRepo.FindByID(ctx, userID)
}

// ConfirmSubscriptionPayment consulta directamente a MP el estado de un pago (sin depender
// del webhook, útil cuando el backend no es alcanzable públicamente) y activa el plan Pro
// si el pago está aprobado y pertenece a este mismo usuario.
func (s *authService) ConfirmSubscriptionPayment(ctx context.Context, userID uuid.UUID, paymentID string) error {
	if s.paymentProvider == nil {
		return fmt.Errorf("%w: pasarela de pagos no configurada", ErrPaymentFailed)
	}

	info, err := s.paymentProvider.GetPaymentInfo(ctx, paymentID)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrPaymentFailed, err)
	}

	if info.ExternalRef != SubscriptionExternalRefPrefix+userID.String() {
		return ErrPaymentRefMismatch
	}

	if info.Status != "approved" {
		return ErrPaymentNotApproved
	}

	return s.userRepo.UpdateIsPro(ctx, userID, true)
}
