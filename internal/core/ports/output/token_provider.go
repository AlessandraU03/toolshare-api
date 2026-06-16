package output

import (
	"github.com/google/uuid"
	"github.com/yourusername/tool-inventory-api/internal/core/domain"
)

type TokenProvider interface {
	Generate(userID uuid.UUID, role domain.Role) (string, error)
	Validate(token string) (uuid.UUID, domain.Role, error)
}
