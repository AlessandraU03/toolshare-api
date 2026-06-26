package rentaldomain

import (
	"time"
	"github.com/google/uuid"
)

type Message struct {
	ID        uuid.UUID
	RentalID  uuid.UUID
	SenderID  uuid.UUID
	Message   string
	CreatedAt time.Time
}
