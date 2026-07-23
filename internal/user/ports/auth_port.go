package userports

import (
	"context"
	"io"

	"github.com/google/uuid"
	sharedports "github.com/yourusername/tool-inventory-api/internal/shared/ports"
	userdomain "github.com/yourusername/tool-inventory-api/internal/user/domain"
)

type RegisterInput struct {
	Name     string
	Email    string
	Password string
	Role     userdomain.Role
	Phone    string
	INE      string
}

type AuthOutput struct {
	Token string
	User  *userdomain.User
}

type AuthService interface {
	Register(ctx context.Context, inp RegisterInput) (*AuthOutput, error)
	Login(ctx context.Context, email, password string) (*AuthOutput, error)

	// CreateSubscriptionPreference crea una preferencia de Checkout Pro en MP
	// para que el usuario pague la suscripción Pro mensual.
	CreateSubscriptionPreference(ctx context.Context, userID uuid.UUID) (sharedports.CreatePreferenceOutput, error)

	// GetProfile devuelve el usuario actual (usado para refrescar is_pro tras el pago).
	GetProfile(ctx context.Context, userID uuid.UUID) (*userdomain.User, error)

	// ConfirmSubscriptionPayment verifica el pago directamente contra MP y activa el plan Pro.
	ConfirmSubscriptionPayment(ctx context.Context, userID uuid.UUID, paymentID string) error

	// VerifyKyc se encarga de procesar los archivos de identificación oficial y selfie del usuario
	VerifyKyc(ctx context.Context, ineFilename string, ine io.Reader, selfieFilename string, selfie io.Reader, curp string) (interface{}, error)
	// StartKycJob / GetKycJob: versión asíncrona de VerifyKyc para evitar que
	// una petición HTTP se quede abierta tanto tiempo que algún proxy
	// intermedio la corte a medias (ver kyc_job_store.go).
	StartKycJob(ineFilename string, ineContent []byte, selfieFilename string, selfieContent []byte, curp string) string
	GetKycJob(jobID string) (status string, result interface{}, errMsg string, found bool)

	// AddCard tokeniza (ya hecho en el cliente) y guarda una tarjeta en el Customer de MP del usuario.
	AddCard(ctx context.Context, userID uuid.UUID, cardToken string) (*userdomain.SavedCard, error)
	ListCards(ctx context.Context, userID uuid.UUID) ([]*userdomain.SavedCard, error)
	DeleteCard(ctx context.Context, userID uuid.UUID, cardID uuid.UUID) error

	// ── Marketplace / OAuth Connect (vínculo de cuenta MP del propietario) ──

	// StartMPConnect genera la URL de autorización de Mercado Pago para que
	// el propietario vincule su propia cuenta.
	StartMPConnect(ctx context.Context, userID uuid.UUID) (authURL string, err error)

	// HandleMPConnectCallback procesa el "code" recibido tras la autorización
	// y guarda los tokens de la cuenta del propietario.
	HandleMPConnectCallback(ctx context.Context, code, state string) error

	// GetMPConnectStatus indica si el usuario ya vinculó su cuenta MP.
	GetMPConnectStatus(ctx context.Context, userID uuid.UUID) (connected bool, err error)
}
