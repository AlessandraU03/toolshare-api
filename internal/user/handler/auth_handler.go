package userhandler

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	userports "github.com/yourusername/tool-inventory-api/internal/user/ports"
	userservice "github.com/yourusername/tool-inventory-api/internal/user/service"
)

type AuthHandler struct {
	authSvc userports.AuthService
}

func NewAuthHandler(authSvc userports.AuthService) *AuthHandler {
	return &AuthHandler{authSvc: authSvc}
}

// Register godoc
// @Summary      Registrar usuario
// @Description  Crea una cuenta de Propietario (owner) o Solicitante (requester) y devuelve un JWT
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body RegisterRequest true "Datos de registro"
// @Success      201 {object} AuthResponse
// @Failure      400 {object} dto.ErrResponse
// @Failure      409 {object} dto.ErrResponse "Email ya registrado"
// @Failure      500 {object} dto.ErrResponse
// @Router       /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	out, err := h.authSvc.Register(c.Request.Context(), userports.RegisterInput{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
		Role:     req.Role,
		Phone:    req.Phone,
		INE:      req.INE,
	})
	if err != nil {
		switch {
		case errors.Is(err, userservice.ErrEmailAlreadyExists):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			log.Printf("ERROR REGISTRANDO USUARIO: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error interno del servidor"})
		}
		return
	}

	c.JSON(http.StatusCreated, ToAuthResponse(out))
}

// Login godoc
// @Summary      Iniciar sesión
// @Description  Autentica al usuario y devuelve un JWT para usar en endpoints protegidos
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body LoginRequest true "Credenciales"
// @Success      200 {object} AuthResponse
// @Failure      400 {object} dto.ErrResponse
// @Failure      401 {object} dto.ErrResponse "Credenciales inválidas"
// @Failure      500 {object} dto.ErrResponse
// @Router       /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	out, err := h.authSvc.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, userservice.ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		log.Printf("ERROR INICIANDO SESIÓN: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error interno del servidor"})
		return
	}

	c.JSON(http.StatusOK, ToAuthResponse(out))
}
