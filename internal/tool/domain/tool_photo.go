package tooldomain

import (
	"time"

	"github.com/google/uuid"
)

// ToolPhoto es una foto individual de una herramienta con su propio score de
// condición (verificado por la CNN en el servidor, nunca por el cliente).
// tools.condition_score se deriva como el MINIMO de todas las ToolPhoto de
// esa herramienta — ver MinRequiredPhotos y el uso en tool_service.go.
type ToolPhoto struct {
	ID             int
	ToolID         uuid.UUID
	PhotoURL       string
	ConditionScore float64
	CreatedAt      time.Time
}
