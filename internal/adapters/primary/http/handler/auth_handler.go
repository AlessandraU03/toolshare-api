package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/tool-inventory-api/internal/adapters/primary/http/dto"
	"github.com/yourusername/tool-inventory-api/internal/core/ports/input"
	"github.com/yourusername/tool-inventory-api/internal/core/service"
)

type AuthHandler struct {
	authSvc input.AuthService
}

func NewAuthHandler(authSvc input.AuthService) *AuthHandler {
	return &AuthHandler{authSvc: authSvc}
}

// Register godoc
// @Summary      Registrar usuario
// @Description  Crea una cuenta de Propietario (owner) o Solicitante (requester) y devuelve un JWT
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body dto.RegisterRequest true "Datos de registro"
// @Success      201 {object} dto.AuthResponse
// @Failure      400 {object} dto.ErrResponse
// @Failure      409 {object} dto.ErrResponse "Email ya registrado"
// @Failure      500 {object} dto.ErrResponse
// @Router       /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	out, err := h.authSvc.Register(c.Request.Context(), input.RegisterInput{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
		Role:     req.Role,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrEmailAlreadyExists):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error interno del servidor"})
		}
		return
	}

	c.JSON(http.StatusCreated, dto.ToAuthResponse(out))
}

// Login godoc
// @Summary      Iniciar sesión
// @Description  Autentica al usuario y devuelve un JWT para usar en endpoints protegidos
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body dto.LoginRequest true "Credenciales"
// @Success      200 {object} dto.AuthResponse
// @Failure      400 {object} dto.ErrResponse
// @Failure      401 {object} dto.ErrResponse "Credenciales inválidas"
// @Failure      500 {object} dto.ErrResponse
// @Router       /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	out, err := h.authSvc.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error interno del servidor"})
		return
	}

	c.JSON(http.StatusOK, dto.ToAuthResponse(out))
}
