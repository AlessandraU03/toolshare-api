package domain

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleOwner     Role = "owner"
	RoleRequester Role = "requester"
)

type User struct {
	ID        uuid.UUID
	Name      string
	Email     string
	Password  string
	Role      Role
	CreatedAt time.Time
	UpdatedAt time.Time
}
