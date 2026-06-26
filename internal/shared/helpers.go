package shared

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	apperrors "github.com/yourusername/tool-inventory-api/internal/shared/errors"
)

func ParseUUID(c *gin.Context, param string) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param(param))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "el ID no es un UUID válido"})
		return uuid.Nil, err
	}
	return id, nil
}

func HandleServiceErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, apperrors.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "recurso no encontrado"})
	case errors.Is(err, apperrors.ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": "no tienes permiso sobre este recurso"})
	default:
		// Detectar error de llave foránea de PostgreSQL (herramienta con rentas activas)
		if strings.Contains(err.Error(), "foreign key") || strings.Contains(err.Error(), "violates foreign key constraint") {
			c.JSON(http.StatusConflict, gin.H{"error": "No se puede eliminar: la herramienta tiene rentas asociadas"})
			return
		}
		log.Printf("ERROR no manejado en handler: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
