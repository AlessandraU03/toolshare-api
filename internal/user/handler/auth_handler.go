package userhandler

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	sharedmiddleware "github.com/yourusername/tool-inventory-api/internal/shared/middleware"
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

// SubscribePreference godoc
// @Summary      Crear preferencia de pago para el plan Pro
// @Description  Crea una preferencia en Mercado Pago para la suscripción mensual y devuelve el init_point para cargar en el WebView del frontend
// @Tags         auth
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} SubscribePreferenceResponse
// @Failure      401 {object} dto.ErrResponse
// @Failure      422 {object} dto.ErrResponse "Error al crear preferencia en MP"
// @Router       /auth/subscribe/preference [post]
func (h *AuthHandler) SubscribePreference(c *gin.Context) {
	userID := sharedmiddleware.UserIDFromContext(c)

	out, err := h.authSvc.CreateSubscriptionPreference(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, userservice.ErrPaymentFailed) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
		log.Printf("ERROR CREANDO PREFERENCIA DE SUSCRIPCIÓN: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error interno del servidor"})
		return
	}

	c.JSON(http.StatusOK, SubscribePreferenceResponse{
		InitPoint:    out.InitPoint,
		PreferenceID: out.PreferenceID,
	})
}

// ConfirmSubscription godoc
// @Summary      Confirmar pago de suscripción Pro
// @Description  Verifica el estado de un pago directamente contra Mercado Pago y activa el plan Pro si está aprobado. Útil cuando el webhook no puede alcanzar al backend (dev local).
// @Tags         auth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body ConfirmSubscriptionRequest true "ID del pago devuelto por MP"
// @Success      200 {object} UserResponse
// @Failure      401 {object} dto.ErrResponse
// @Failure      402 {object} dto.ErrResponse "Pago no aprobado o no pertenece al usuario"
// @Router       /auth/subscribe/confirm [post]
func (h *AuthHandler) ConfirmSubscription(c *gin.Context) {
	userID := sharedmiddleware.UserIDFromContext(c)

	var req ConfirmSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.authSvc.ConfirmSubscriptionPayment(c.Request.Context(), userID, req.PaymentID); err != nil {
		switch {
		case errors.Is(err, userservice.ErrPaymentNotApproved), errors.Is(err, userservice.ErrPaymentRefMismatch):
			c.JSON(http.StatusPaymentRequired, gin.H{"error": err.Error()})
		default:
			log.Printf("ERROR CONFIRMANDO PAGO DE SUSCRIPCIÓN: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error interno del servidor"})
		}
		return
	}

	user, err := h.authSvc.GetProfile(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"is_pro": true})
		return
	}

	c.JSON(http.StatusOK, ToUserResponse(user))
}

// Me godoc
// @Summary      Perfil del usuario autenticado
// @Description  Devuelve los datos actuales del usuario (útil para refrescar is_pro tras un pago)
// @Tags         auth
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} UserResponse
// @Failure      401 {object} dto.ErrResponse
// @Router       /auth/me [get]
func (h *AuthHandler) Me(c *gin.Context) {
	userID := sharedmiddleware.UserIDFromContext(c)

	user, err := h.authSvc.GetProfile(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuario no encontrado"})
		return
	}

	c.JSON(http.StatusOK, ToUserResponse(user))
}

// VerifyKyc maneja la verificación de identidad mediante carga de archivos (INE y Selfie)
func (h *AuthHandler) VerifyKyc(c *gin.Context) {
	ineFile, ineHeader, err := c.Request.FormFile("ine_image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "se requiere la foto del INE ('ine_image')"})
		return
	}
	defer ineFile.Close()

	selfieFile, selfieHeader, err := c.Request.FormFile("selfie_image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "se requiere la foto selfie ('selfie_image')"})
		return
	}
	defer selfieFile.Close()

	res, err := h.authSvc.VerifyKyc(c.Request.Context(), ineHeader.Filename, ineFile, selfieHeader.Filename, selfieFile)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}
