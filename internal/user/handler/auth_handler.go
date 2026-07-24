package userhandler

import (
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	apperrors "github.com/yourusername/tool-inventory-api/internal/shared/errors"
	sharedmiddleware "github.com/yourusername/tool-inventory-api/internal/shared/middleware"
	userdomain "github.com/yourusername/tool-inventory-api/internal/user/domain"
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

// SaveBankAccount godoc
// @Summary      Guardar datos bancarios
// @Description  Guarda la CLABE, titular y banco del propietario. Se usan para que el administrador le transfiera manualmente el pago de una disputa ganada con seguro activo (Mercado Pago no ofrece una API de transferencia directa con esta integración)
// @Tags         auth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body BankAccountRequest true "Datos bancarios"
// @Success      200 {object} BankAccountResponse
// @Failure      400 {object} dto.ErrResponse
// @Failure      401 {object} dto.ErrResponse
// @Router       /auth/bank-account [put]
func (h *AuthHandler) SaveBankAccount(c *gin.Context) {
	var req BankAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := sharedmiddleware.UserIDFromContext(c)
	account := userdomain.BankAccount{
		CLABE:         req.CLABE,
		AccountHolder: req.AccountHolder,
		BankName:      req.BankName,
	}
	if err := h.authSvc.SaveBankAccount(c.Request.Context(), userID, account); err != nil {
		if errors.Is(err, userservice.ErrInvalidCLABE) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error interno del servidor"})
		return
	}

	c.JSON(http.StatusOK, ToBankAccountResponse(&account))
}

// GetBankAccount godoc
// @Summary      Consultar datos bancarios
// @Description  Devuelve los datos bancarios registrados del usuario autenticado
// @Tags         auth
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} BankAccountResponse
// @Failure      401 {object} dto.ErrResponse
// @Router       /auth/bank-account [get]
func (h *AuthHandler) GetBankAccount(c *gin.Context) {
	userID := sharedmiddleware.UserIDFromContext(c)
	account, err := h.authSvc.GetBankAccount(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error interno del servidor"})
		return
	}
	c.JSON(http.StatusOK, ToBankAccountResponse(account))
}

// DeleteBankAccount godoc
// @Summary      Eliminar datos bancarios
// @Description  Borra la CLABE, titular y banco registrados por el propietario
// @Tags         auth
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} dto.MsgResponse
// @Failure      401 {object} dto.ErrResponse
// @Failure      404 {object} dto.ErrResponse
// @Router       /auth/bank-account [delete]
func (h *AuthHandler) DeleteBankAccount(c *gin.Context) {
	userID := sharedmiddleware.UserIDFromContext(c)

	if err := h.authSvc.DeleteBankAccount(c.Request.Context(), userID); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "usuario no encontrado"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error al eliminar los datos bancarios"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "datos bancarios eliminados correctamente"})
}

// AddCard godoc
// @Summary      Guardar tarjeta
// @Description  Guarda una tarjeta a partir de un card_token tokenizado en el cliente contra la API pública de Mercado Pago (POST /v1/card_tokens)
// @Tags         auth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body AddCardRequest true "Token de tarjeta"
// @Success      201 {object} SavedCardResponse
// @Failure      400 {object} dto.ErrResponse
// @Failure      401 {object} dto.ErrResponse
// @Failure      422 {object} dto.ErrResponse "Mercado Pago rechazó la tarjeta"
// @Router       /auth/cards [post]
func (h *AuthHandler) AddCard(c *gin.Context) {
	userID := sharedmiddleware.UserIDFromContext(c)

	var req AddCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	card, err := h.authSvc.AddCard(c.Request.Context(), userID, req.CardToken)
	if err != nil {
		if errors.Is(err, userservice.ErrCardFailed) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
		log.Printf("ERROR GUARDANDO TARJETA: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error interno del servidor"})
		return
	}

	c.JSON(http.StatusCreated, ToSavedCardResponse(card))
}

// ListCards godoc
// @Summary      Listar tarjetas guardadas
// @Tags         auth
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} SavedCardResponse
// @Failure      401 {object} dto.ErrResponse
// @Router       /auth/cards [get]
func (h *AuthHandler) ListCards(c *gin.Context) {
	userID := sharedmiddleware.UserIDFromContext(c)

	cards, err := h.authSvc.ListCards(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error al listar tarjetas"})
		return
	}

	c.JSON(http.StatusOK, ToSavedCardListResponse(cards))
}

// DeleteCard godoc
// @Summary      Eliminar tarjeta guardada
// @Tags         auth
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "UUID de la tarjeta"
// @Success      200 {object} dto.MsgResponse
// @Failure      401 {object} dto.ErrResponse
// @Failure      404 {object} dto.ErrResponse
// @Router       /auth/cards/{id} [delete]
func (h *AuthHandler) DeleteCard(c *gin.Context) {
	userID := sharedmiddleware.UserIDFromContext(c)

	cardID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id de tarjeta inválido"})
		return
	}

	if err := h.authSvc.DeleteCard(c.Request.Context(), userID, cardID); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "tarjeta no encontrada"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error al eliminar la tarjeta"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "tarjeta eliminada correctamente"})
}

// VerifyKyc maneja la verificación de identidad mediante carga de archivos
// (INE y Selfie). Asíncrono a propósito: puede tardar 15-20+ segundos (Haar
// Cascade + arranque en frío del worker de PaddleOCR + ArcFace), y una sola
// petición HTTP tan larga corre el riesgo de que algún proxy intermedio la
// corte a medias aunque el servidor sí termine bien (confirmado en
// producción). Responde de inmediato con un job_id; ver GetKycJob para el
// resultado.
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

	curp := c.PostForm("curp")

	ineBytes, err := io.ReadAll(ineFile)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no se pudo leer la foto del INE"})
		return
	}
	selfieBytes, err := io.ReadAll(selfieFile)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no se pudo leer la foto selfie"})
		return
	}

	jobID := h.authSvc.StartKycJob(ineHeader.Filename, ineBytes, selfieHeader.Filename, selfieBytes, curp)
	c.JSON(http.StatusAccepted, gin.H{"job_id": jobID})
}

// GetKycJob consulta el resultado de una verificación KYC iniciada con
// VerifyKyc. El cliente pregunta cada pocos segundos hasta que status ya
// no sea "processing".
func (h *AuthHandler) GetKycJob(c *gin.Context) {
	jobID := c.Param("job_id")

	status, result, errMsg, found := h.authSvc.GetKycJob(jobID)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "job no encontrado o expirado"})
		return
	}

	resp := gin.H{"status": status}
	if errMsg != "" {
		resp["error"] = errMsg
	}
	if status == "done" {
		if resultMap, ok := result.(map[string]interface{}); ok {
			for k, v := range resultMap {
				resp[k] = v
			}
		}
	}
	c.JSON(http.StatusOK, resp)
}

// StartMPConnect godoc
// @Summary      Iniciar vínculo de cuenta de Mercado Pago (Marketplace)
// @Description  Devuelve la URL de autorización de MP que el propietario debe abrir para conectar su propia cuenta
// @Tags         auth
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} map[string]string
// @Router       /auth/mp-connect/start [get]
func (h *AuthHandler) StartMPConnect(c *gin.Context) {
	userID := sharedmiddleware.UserIDFromContext(c)

	authURL, err := h.authSvc.StartMPConnect(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"auth_url": authURL})
}

// GetMPConnectStatus godoc
// @Summary      Estado del vínculo de cuenta de Mercado Pago
// @Tags         auth
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} map[string]bool
// @Router       /auth/mp-connect/status [get]
func (h *AuthHandler) GetMPConnectStatus(c *gin.Context) {
	userID := sharedmiddleware.UserIDFromContext(c)

	connected, err := h.authSvc.GetMPConnectStatus(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"connected": connected})
}

// MPConnectCallback godoc
// @Summary      Callback de OAuth de Mercado Pago (lo invoca MP, no el cliente)
// @Tags         auth
// @Produce      html
// @Param        code  query string true "Código de autorización"
// @Param        state query string true "State generado en /auth/mp-connect/start"
// @Success      200 {string} string "Página HTML de confirmación"
// @Router       /auth/mp-connect/callback [get]
func (h *AuthHandler) MPConnectCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	err := h.authSvc.HandleMPConnectCallback(c.Request.Context(), code, state)
	if err != nil {
		log.Printf("ERROR MP CONNECT CALLBACK: %v", err)
		c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(
			`<html><body style="font-family:sans-serif;text-align:center;padding:40px">`+
				`<h2>No se pudo vincular tu cuenta</h2><p>Vuelve a intentarlo desde la app.</p></body></html>`))
		return
	}

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(
		`<html><body style="font-family:sans-serif;text-align:center;padding:40px">`+
			`<h2>Cuenta de Mercado Pago vinculada ✅</h2><p>Ya puedes cerrar esta ventana.</p></body></html>`))
}
