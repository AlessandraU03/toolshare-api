package supportports

import (
	"context"
	"time"

	"github.com/google/uuid"
	supportdomain "github.com/yourusername/tool-inventory-api/internal/support/domain"
)

// ThreadSummary describe un hilo de soporte para la lista que ve el admin:
// el propietario y una vista previa de su último mensaje.
type ThreadSummary struct {
	OwnerID       uuid.UUID
	OwnerName     string
	LastMessage   string
	LastMessageAt time.Time
}

type SupportRepository interface {
	Create(ctx context.Context, msg *supportdomain.SupportMessage) (*supportdomain.SupportMessage, error)
	ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]*supportdomain.SupportMessage, error)
	// ListThreads devuelve un hilo por cada propietario que tenga al menos un
	// mensaje, ordenados por actividad más reciente primero.
	ListThreads(ctx context.Context) ([]ThreadSummary, error)
}

type SupportService interface {
	// GetOwnerThread devuelve los mensajes del hilo de soporte del propietario autenticado.
	GetOwnerThread(ctx context.Context, ownerID uuid.UUID) ([]*supportdomain.SupportMessage, error)
	// SendAsOwner agrega un mensaje al hilo del propietario autenticado.
	SendAsOwner(ctx context.Context, ownerID uuid.UUID, message string) (*supportdomain.SupportMessage, error)
	// GetThreadForAdmin devuelve los mensajes del hilo de un propietario específico.
	GetThreadForAdmin(ctx context.Context, ownerID uuid.UUID) ([]*supportdomain.SupportMessage, error)
	// SendAsAdmin agrega un mensaje al hilo de un propietario, en nombre del administrador autenticado.
	SendAsAdmin(ctx context.Context, ownerID, adminID uuid.UUID, message string) (*supportdomain.SupportMessage, error)
	// ListThreads lista todos los hilos con actividad, para el panel de administración.
	ListThreads(ctx context.Context) ([]ThreadSummary, error)
}
