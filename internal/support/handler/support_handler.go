package supporthandler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	apperrors "github.com/yourusername/tool-inventory-api/internal/shared/errors"
	sharedmiddleware "github.com/yourusername/tool-inventory-api/internal/shared/middleware"
	supportports "github.com/yourusername/tool-inventory-api/internal/support/ports"
	supportservice "github.com/yourusername/tool-inventory-api/internal/support/service"
)

type SupportHandler struct {
	supportSvc supportports.SupportService
}

func NewSupportHandler(supportSvc supportports.SupportService) *SupportHandler {
	return &SupportHandler{supportSvc: supportSvc}
}

// GetMyMessages godoc
// @Summary      Mensajes del hilo de soporte del propietario autenticado
// @Tags         soporte
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} SupportMessageResponse
// @Router       /support/messages [get]
func (h *SupportHandler) GetMyMessages(c *gin.Context) {
	ownerID := sharedmiddleware.UserIDFromContext(c)
	msgs, err := h.supportSvc.GetOwnerThread(c.Request.Context(), ownerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error al consultar los mensajes"})
		return
	}
	c.JSON(http.StatusOK, ToSupportMessageListResponse(msgs))
}

// SendMyMessage godoc
// @Summary      Enviar un mensaje en el hilo de soporte propio
// @Tags         soporte
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body SendSupportMessageRequest true "Mensaje"
// @Success      201 {object} SupportMessageResponse
// @Router       /support/messages [post]
func (h *SupportHandler) SendMyMessage(c *gin.Context) {
	ownerID := sharedmiddleware.UserIDFromContext(c)

	var req SendSupportMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	msg, err := h.supportSvc.SendAsOwner(c.Request.Context(), ownerID, req.Message)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, ToSupportMessageResponse(msg))
}

// ListThreads godoc
// @Summary      Listar hilos de soporte con actividad (uso del administrador)
// @Tags         soporte
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} SupportThreadResponse
// @Router       /admin/support/threads [get]
func (h *SupportHandler) ListThreads(c *gin.Context) {
	threads, err := h.supportSvc.ListThreads(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error al listar los hilos de soporte"})
		return
	}
	c.JSON(http.StatusOK, ToSupportThreadListResponse(threads))
}

// GetThreadMessages godoc
// @Summary      Mensajes del hilo de soporte de un propietario (uso del administrador)
// @Tags         soporte
// @Produce      json
// @Param        ownerId path string true "UUID del propietario"
// @Security     BearerAuth
// @Success      200 {array} SupportMessageResponse
// @Router       /admin/support/threads/{ownerId}/messages [get]
func (h *SupportHandler) GetThreadMessages(c *gin.Context) {
	ownerID, err := uuid.Parse(c.Param("ownerId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de propietario inválido"})
		return
	}

	msgs, err := h.supportSvc.GetThreadForAdmin(c.Request.Context(), ownerID)
	if err != nil {
		h.handleThreadErr(c, err)
		return
	}
	c.JSON(http.StatusOK, ToSupportMessageListResponse(msgs))
}

// SendThreadMessage godoc
// @Summary      Responder en el hilo de soporte de un propietario (uso del administrador)
// @Tags         soporte
// @Accept       json
// @Produce      json
// @Param        ownerId path string true "UUID del propietario"
// @Param        body body SendSupportMessageRequest true "Mensaje"
// @Security     BearerAuth
// @Success      201 {object} SupportMessageResponse
// @Router       /admin/support/threads/{ownerId}/messages [post]
func (h *SupportHandler) SendThreadMessage(c *gin.Context) {
	ownerID, err := uuid.Parse(c.Param("ownerId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de propietario inválido"})
		return
	}

	var req SendSupportMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	adminID := sharedmiddleware.UserIDFromContext(c)
	msg, err := h.supportSvc.SendAsAdmin(c.Request.Context(), ownerID, adminID, req.Message)
	if err != nil {
		h.handleThreadErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, ToSupportMessageResponse(msg))
}

func (h *SupportHandler) handleThreadErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, apperrors.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "propietario no encontrado"})
	case errors.Is(err, supportservice.ErrNotAnOwner):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, supportservice.ErrEmptyMessage):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error interno del servidor"})
	}
}
