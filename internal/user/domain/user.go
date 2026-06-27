package userdomain

import (
	"time"
	"github.com/google/uuid"
)

type Role string

const (
	RoleOwner     Role = "owner"
	RoleRequester Role = "requester"
	RoleAdmin     Role = "admin"
)

type User struct {
	ID        uuid.UUID
	Name      string
	Email     string
	Password  string
	Role      Role
	IsPro     bool
	Phone     string
	INE       string
	CreatedAt time.Time
	UpdatedAt time.Time
}
