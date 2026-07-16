package reviewdomain

import (
	"time"

	"github.com/google/uuid"
)

type TargetType string

const (
	// TargetTypeTool: reseña del solicitante hacia la herramienta rentada.
	TargetTypeTool TargetType = "tool"
	// TargetTypeUser: calificación del propietario hacia el solicitante.
	TargetTypeUser TargetType = "user"
)

type Review struct {
	ID         uuid.UUID
	RentalID   uuid.UUID
	AuthorID   uuid.UUID
	TargetType TargetType
	TargetID   uuid.UUID
	Rating     int
	Comment    string
	CreatedAt  time.Time
}
