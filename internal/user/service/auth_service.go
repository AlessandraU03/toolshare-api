package userservice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

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
	ErrCardFailed         = errors.New("no se pudo guardar la tarjeta")
)

// SubscriptionExternalRefPrefix marca las external_reference de MP que corresponden
// a una suscripción Pro (en vez de una renta), para distinguirlas en el webhook.
const SubscriptionExternalRefPrefix = "sub:"

// ProMonthlyPrice es el precio mensual del plan Pro en MXN.
// TODO: temporalmente en $20 para pruebas — regresar a 69.0 antes de producción real.
// $1 no funcionaba: Mercado Pago México exige mínimo $5-10 MXN para pagos con tarjeta.
const ProMonthlyPrice = 20.0

type authService struct {
	userRepo        userports.UserRepository
	tokenProvider   sharedports.TokenProvider
	paymentProvider sharedports.PaymentProvider
	kycJobs         *kycJobStore
}

func NewAuthService(userRepo userports.UserRepository, tokenProvider sharedports.TokenProvider, paymentProvider sharedports.PaymentProvider) userports.AuthService {
	return &authService{userRepo: userRepo, tokenProvider: tokenProvider, paymentProvider: paymentProvider, kycJobs: newKycJobStore()}
}

// StartKycJob recibe los bytes de INE y selfie ya leidos y arranca la
// verificacion KYC de fondo, devolviendo un job_id de inmediato en vez de
// bloquear la peticion HTTP (ver kyc_job_store.go).
func (s *authService) StartKycJob(ineFilename string, ineContent []byte, selfieFilename string, selfieContent []byte, curp string) string {
	return s.kycJobs.start(s, ineFilename, ineContent, selfieFilename, selfieContent, curp)
}

// GetKycJob consulta el estatus de un job de KYC ya arrancado con StartKycJob.
func (s *authService) GetKycJob(jobID string) (status string, result interface{}, errMsg string, found bool) {
	job, ok := s.kycJobs.get(jobID)
	if !ok {
		return "", nil, "", false
	}
	return string(job.Status), job.Result, job.Error, true
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
		return nil, ErrInvalidCredentials
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
		// auto_return de MP exige una URL absoluta http(s); "toolshare://" no
		// es válida y deja el botón "Pagar" inerte. El WebView de Flutter
		// intercepta esta ruta antes de que la navegación se complete.
		backURL = "https://toolshare-api.up.railway.app/payment"
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

func (s *authService) VerifyKyc(ctx context.Context, ineFilename string, ine io.Reader, selfieFilename string, selfie io.Reader, curp string) (interface{}, error) {
	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)

	// Crear campo "ine_image"
	ineWriter, err := bodyWriter.CreateFormFile("ine_image", ineFilename)
	if err != nil {
		return nil, fmt.Errorf("crear campo ine_image: %w", err)
	}
	if _, err := io.Copy(ineWriter, ine); err != nil {
		return nil, fmt.Errorf("copiar ine_image a multipart: %w", err)
	}

	// Crear campo "selfie_image"
	selfieWriter, err := bodyWriter.CreateFormFile("selfie_image", selfieFilename)
	if err != nil {
		return nil, fmt.Errorf("crear campo selfie_image: %w", err)
	}
	if _, err := io.Copy(selfieWriter, selfie); err != nil {
		return nil, fmt.Errorf("copiar selfie_image a multipart: %w", err)
	}

	// Agregar campo "curp" si existe
	if curp != "" {
		if err := bodyWriter.WriteField("curp", curp); err != nil {
			return nil, fmt.Errorf("escribir campo curp: %w", err)
		}
	}

	bodyWriter.Close()

	mlBaseURL := os.Getenv("ML_SERVICE_URL")
	if mlBaseURL == "" {
		mlBaseURL = "http://localhost:8000"
	}
	apiURL := mlBaseURL + "/verify-kyc"

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bodyBuf)
	if err != nil {
		return nil, fmt.Errorf("crear request KYC a ML: %w", err)
	}

	req.Header.Set("Content-Type", bodyWriter.FormDataContentType())

	// 90s, no 10s: /verify-kyc hace Haar Cascade + arranque del worker
	// aislado de PaddleOCR (cold start puede tardar decenas de segundos) +
	// carga e inferencia de ArcFace para la comparación facial real. Con un
	// timeout corto, la primera verificación después de que el servicio de
	// ML arranca (o tras estar inactivo un rato) fallaba con "context
	// deadline exceeded" antes de que terminara de calentar.
	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ejecutar request KYC a ML: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("el validador de identidad ML reportó un error (%d): %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decodificar respuesta KYC: %w", err)
	}

	return result, nil
}

// AddCard guarda una tarjeta tokenizada en el Customer de Mercado Pago del
// usuario, creando el Customer si aún no existe.
func (s *authService) AddCard(ctx context.Context, userID uuid.UUID, cardToken string) (*userdomain.SavedCard, error) {
	if s.paymentProvider == nil {
		return nil, fmt.Errorf("%w: pasarela de pagos no configurada", ErrCardFailed)
	}

	customerID, err := s.userRepo.GetMPCustomerID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if customerID == "" {
		user, err := s.userRepo.FindByID(ctx, userID)
		if err != nil {
			return nil, err
		}
		customerID, err = s.paymentProvider.CreateCustomer(ctx, user.Email)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrCardFailed, err)
		}
		if err := s.userRepo.SetMPCustomerID(ctx, userID, customerID); err != nil {
			return nil, err
		}
	}

	saved, err := s.paymentProvider.SaveCard(ctx, customerID, cardToken)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCardFailed, err)
	}

	card := &userdomain.SavedCard{
		UserID:          userID,
		MPCardID:        saved.MPCardID,
		CardBrand:       saved.CardBrand,
		LastFourDigits:  saved.LastFourDigits,
		ExpirationMonth: saved.ExpirationMonth,
		ExpirationYear:  saved.ExpirationYear,
	}
	return s.userRepo.SaveCard(ctx, card)
}

func (s *authService) ListCards(ctx context.Context, userID uuid.UUID) ([]*userdomain.SavedCard, error) {
	return s.userRepo.ListCards(ctx, userID)
}

func (s *authService) DeleteCard(ctx context.Context, userID uuid.UUID, cardID uuid.UUID) error {
	mpCardID, err := s.userRepo.DeleteCard(ctx, userID, cardID)
	if err != nil {
		return err
	}
	if s.paymentProvider == nil || mpCardID == "" {
		return nil
	}

	customerID, err := s.userRepo.GetMPCustomerID(ctx, userID)
	if err != nil || customerID == "" {
		return nil
	}
	// La tarjeta ya se eliminó localmente; si MP falla solo queda huérfana en su lado.
	_ = s.paymentProvider.DeleteCard(ctx, customerID, mpCardID)
	return nil
}

// mpConnectStatePrefix distingue el "state" de OAuth Connect de un JWT de sesión normal.
const mpConnectStatePrefix = "mpconnect:"

// StartMPConnect genera la URL de autorización de Mercado Pago para que el
// propietario vincule su propia cuenta. El "state" es un JWT de corta vida
// (reusa el mismo TokenProvider de sesión) que amarra el callback al userID.
func (s *authService) StartMPConnect(ctx context.Context, userID uuid.UUID) (string, error) {
	if s.paymentProvider == nil {
		return "", fmt.Errorf("pasarela de pagos no configurada")
	}
	token, err := s.tokenProvider.Generate(userID, userdomain.RoleOwner)
	if err != nil {
		return "", fmt.Errorf("generar state de OAuth: %w", err)
	}
	return s.paymentProvider.GetOAuthURL(mpConnectStatePrefix + token), nil
}

// HandleMPConnectCallback procesa el "code" recibido tras la autorización y
// guarda los tokens de la cuenta del propietario.
func (s *authService) HandleMPConnectCallback(ctx context.Context, code, state string) error {
	if s.paymentProvider == nil {
		return fmt.Errorf("pasarela de pagos no configurada")
	}
	if !strings.HasPrefix(state, mpConnectStatePrefix) {
		return fmt.Errorf("state de OAuth inválido")
	}
	token := strings.TrimPrefix(state, mpConnectStatePrefix)
	userID, _, err := s.tokenProvider.Validate(token)
	if err != nil {
		return fmt.Errorf("state de OAuth expirado o inválido: %w", err)
	}

	sellerUserID, accessToken, refreshToken, expiresAt, err := s.paymentProvider.ExchangeOAuthCode(ctx, code)
	if err != nil {
		return fmt.Errorf("intercambiar code de OAuth: %w", err)
	}
	return s.userRepo.SetMPSellerAccount(ctx, userID, sellerUserID, accessToken, refreshToken, expiresAt)
}

// GetMPConnectStatus indica si el usuario ya vinculó su cuenta MP.
func (s *authService) GetMPConnectStatus(ctx context.Context, userID uuid.UUID) (bool, error) {
	account, err := s.userRepo.GetMPSellerAccount(ctx, userID)
	if err != nil {
		return false, err
	}
	return account.Connected(), nil
}
