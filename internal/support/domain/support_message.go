package supportdomain

import (
	"time"

	"github.com/google/uuid"
)

// SupportMessage es un mensaje dentro del hilo de soporte de un propietario.
// El hilo entero está identificado por OwnerID; SenderID puede ser el propio
// propietario o cualquier administrador que haya respondido.
type SupportMessage struct {
	ID        uuid.UUID
	OwnerID   uuid.UUID
	SenderID  uuid.UUID
	Message   string
	CreatedAt time.Time
}
