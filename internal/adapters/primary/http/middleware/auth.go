package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yourusername/tool-inventory-api/internal/core/domain"
	"github.com/yourusername/tool-inventory-api/internal/core/ports/output"
)

const (
	ContextUserID   = "userID"
	ContextUserRole = "userRole"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

func RequireAuth(tokenProvider output.TokenProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{Error: "se requiere el header Authorization"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{Error: "formato: Bearer <token>"})
			return
		}

		userID, role, err := tokenProvider.Validate(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{Error: err.Error()})
			return
		}

		c.Set(ContextUserID, userID)
		c.Set(ContextUserRole, role)
		c.Next()
	}
}

func RequireRole(roles ...domain.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get(ContextUserRole)
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{Error: "usuario no autenticado"})
			return
		}

		role, ok := userRole.(domain.Role)
		if !ok {
			c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorResponse{Error: "error al leer el rol"})
			return
		}

		for _, allowed := range roles {
			if role == allowed {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, ErrorResponse{Error: "no tienes permisos para esta acción"})
	}
}

func UserIDFromContext(c *gin.Context) uuid.UUID {
	return c.MustGet(ContextUserID).(uuid.UUID)
}
