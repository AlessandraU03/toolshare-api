// Package middleware contiene los middlewares de Gin para autenticación y autorización.
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/tool-inventory-api/internal/auth"
	"github.com/yourusername/tool-inventory-api/internal/model"
)

const (
	// ContextUserID es la clave para acceder al ID del usuario en el contexto de Gin.
	ContextUserID = "userID"
	// ContextUserRole es la clave para acceder al rol del usuario en el contexto de Gin.
	ContextUserRole = "userRole"
)

// RequireAuth es un middleware que verifica que el request incluya un JWT válido
// en el header Authorization: Bearer <token>.
// Si el token es inválido o falta, responde con 401 Unauthorized.
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, model.ErrorResponse{
				Error: "se requiere el header Authorization",
			})
			return
		}

		// El header debe tener el formato "Bearer <token>"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, model.ErrorResponse{
				Error: "formato de token inválido, se espera: Bearer <token>",
			})
			return
		}

		claims, err := auth.ValidateToken(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, model.ErrorResponse{
				Error: err.Error(),
			})
			return
		}

		// Inyectar los datos del usuario en el contexto para handlers posteriores
		c.Set(ContextUserID, claims.UserID)
		c.Set(ContextUserRole, claims.Role)
		c.Next()
	}
}

// RequireRole es un middleware de autorización que permite el acceso
// solo a usuarios con alguno de los roles especificados.
// Debe usarse después de RequireAuth().
func RequireRole(roles ...model.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get(ContextUserRole)
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, model.ErrorResponse{
				Error: "usuario no autenticado",
			})
			return
		}

		role, ok := userRole.(model.Role)
		if !ok {
			c.AbortWithStatusJSON(http.StatusInternalServerError, model.ErrorResponse{
				Error: "error al leer el rol del usuario",
			})
			return
		}

		for _, allowed := range roles {
			if role == allowed {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, model.ErrorResponse{
			Error: "no tienes permisos para realizar esta acción",
		})
	}
}
