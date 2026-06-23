package shared

import (
	"errors"
	"net/http"

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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error interno del servidor"})
	}
}
