package rentaldomain

import (
	"time"

	"github.com/google/uuid"
)

type UserFingerprint struct {
	UserID    uuid.UUID
	IPAddress string
	DeviceID  string
	CreatedAt time.Time
}
