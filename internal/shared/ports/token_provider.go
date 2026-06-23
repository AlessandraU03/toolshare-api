package sharedports

import (
	"github.com/google/uuid"
	userdomain "github.com/yourusername/tool-inventory-api/internal/user/domain"
)

type TokenProvider interface {
	Generate(userID uuid.UUID, role userdomain.Role) (string, error)
	Validate(token string) (uuid.UUID, userdomain.Role, error)
}
