package rentalhandler

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	rentaldomain "github.com/yourusername/tool-inventory-api/internal/rental/domain"
	rentalports "github.com/yourusername/tool-inventory-api/internal/rental/ports"
	rentalservice "github.com/yourusername/tool-inventory-api/internal/rental/service"
	"github.com/yourusername/tool-inventory-api/internal/shared"
	sharedmiddleware "github.com/yourusername/tool-inventory-api/internal/shared/middleware"
)

type RentalHandler struct {
	rentalSvc rentalports.RentalService
}

func NewRentalHandler(rentalSvc rentalports.RentalService) *RentalHandler {
	return &RentalHandler{rentalSvc: rentalSvc}
}

// CreateRental godoc
// @Summary      Solicitar renta
// @Description  El solicitante crea una solicitud de renta. Si se envía card_token, los fondos (renta + depósito 10%) quedan congelados en Mercado Pago
// @Tags         rentas
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body CreateRentalRequest true "Datos de la renta"
// @Success      201 {object} RentalResponse
// @Failure      400 {object} dto.ErrResponse
// @Failure      401 {object} dto.ErrResponse
// @Failure      404 {object} dto.ErrResponse "Herramienta no encontrada"
// @Failure      409 {object} dto.ErrResponse "Herramienta no disponible"
// @Failure      422 {object} dto.ErrResponse "Pago rechazado"
// @Router       /rentals [post]
func (h *RentalHandler) CreateRental(c *gin.Context) {
	var req CreateRentalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	toolID, err := uuid.Parse(req.ToolID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tool_id inválido"})
		return
	}

	startDate, err := time.Parse(time.RFC3339, req.StartDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start_date debe estar en formato RFC3339"})
		return
	}
	endDate, err := time.Parse(time.RFC3339, req.EndDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "end_date debe estar en formato RFC3339"})
		return
	}
	if !endDate.After(startDate) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "end_date debe ser posterior a start_date"})
		return
	}

	requesterID := sharedmiddleware.UserIDFromContext(c)

	rental, err := h.rentalSvc.Create(c.Request.Context(), rentalports.CreateRentalInput{
		ToolID:        toolID,
		RequesterID:   requesterID,
		StartDate:     startDate,
		EndDate:       endDate,
		PaymentMethod: req.PaymentMethod,
		CardToken:     req.CardToken,
		PayerEmail:    req.PayerEmail,
	})
	if err != nil {
		switch {
		case errors.Is(err, rentalservice.ErrToolNotAvailable):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case errors.Is(err, rentalservice.ErrPaymentFailed):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		default:
			shared.HandleServiceErr(c, err)
		}
		return
	}

	c.JSON(http.StatusCreated, ToRentalResponse(rental))
}

// GetRentals godoc
// @Summary      Mis rentas
// @Description  Lista todas las rentas donde el usuario es propietario o solicitante
// @Tags         rentas
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array}  RentalResponse
// @Failure      401 {object} dto.ErrResponse
// @Router       /rentals [get]
func (h *RentalHandler) GetRentals(c *gin.Context) {
	userID := sharedmiddleware.UserIDFromContext(c)

	rentals, err := h.rentalSvc.ListByUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error al obtener las rentas"})
		return
	}

	c.JSON(http.StatusOK, ToRentalListResponse(rentals))
}

// GetRental godoc
// @Summary      Detalle de renta
// @Description  Devuelve el estado completo de la renta, incluyendo contrato SHA-256, estado de pago y apretón de manos
// @Tags         rentas
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "UUID de la renta"
// @Success      200 {object} RentalResponse
// @Failure      401 {object} dto.ErrResponse
// @Failure      403 {object} dto.ErrResponse
// @Failure      404 {object} dto.ErrResponse
// @Router       /rentals/{id} [get]
func (h *RentalHandler) GetRental(c *gin.Context) {
	id, err := shared.ParseUUID(c, "id")
	if err != nil {
		return
	}

	rental, err := h.rentalSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		shared.HandleServiceErr(c, err)
		return
	}

	userID := sharedmiddleware.UserIDFromContext(c)
	if rental.OwnerID != userID && rental.RequesterID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "no tienes acceso a esta renta"})
		return
	}

	c.JSON(http.StatusOK, ToRentalResponse(rental))
}

// ConfirmDelivery godoc
// @Summary      Confirmar entrega (apretón de manos)
// @Description  Paso 2: propietario Y solicitante llaman este endpoint con sus coordenadas GPS. Cuando ambos confirman → estado "active" + contrato SHA-256 generado
// @Tags         rentas
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path string                    true  "UUID de la renta"
// @Param        body body ConfirmDeliveryRequest false "Coordenadas GPS (opcional)"
// @Success      200 {object} RentalResponse
// @Failure      401 {object} dto.ErrResponse
// @Failure      403 {object} dto.ErrResponse "No eres parte de esta renta"
// @Failure      404 {object} dto.ErrResponse
// @Failure      409 {object} dto.ErrResponse "Ya confirmaste o renta no está pendiente"
// @Router       /rentals/{id}/confirm-delivery [post]
func (h *RentalHandler) ConfirmDelivery(c *gin.Context) {
	id, err := shared.ParseUUID(c, "id")
	if err != nil {
		return
	}

	// GPS es opcional; ignorar error si no viene body
	var req ConfirmDeliveryRequest
	_ = c.ShouldBindJSON(&req)

	userID := sharedmiddleware.UserIDFromContext(c)

	rental, err := h.rentalSvc.ConfirmDelivery(c.Request.Context(), id, userID, req.Latitude, req.Longitude)
	if err != nil {
		switch {
		case errors.Is(err, rentalservice.ErrUnauthorized):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case errors.Is(err, rentaldomain.ErrRentalNotPending), errors.Is(err, rentaldomain.ErrAlreadyConfirmed):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			shared.HandleServiceErr(c, err)
		}
		return
	}

	c.JSON(http.StatusOK, ToRentalResponse(rental))
}

// ConfirmReturn godoc
// @Summary      Confirmar devolución (apretón de manos)
// @Description  Paso 3: propietario Y solicitante confirman la devolución física. Cuando ambos confirman → "completed", MP cobra la renta y libera el depósito
// @Tags         rentas
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "UUID de la renta"
// @Success      200 {object} RentalResponse
// @Failure      401 {object} dto.ErrResponse
// @Failure      403 {object} dto.ErrResponse
// @Failure      404 {object} dto.ErrResponse
// @Failure      409 {object} dto.ErrResponse "Ya confirmaste o renta no está activa"
// @Router       /rentals/{id}/confirm-return [post]
func (h *RentalHandler) ConfirmReturn(c *gin.Context) {
	id, err := shared.ParseUUID(c, "id")
	if err != nil {
		return
	}

	userID := sharedmiddleware.UserIDFromContext(c)

	rental, err := h.rentalSvc.ConfirmReturn(c.Request.Context(), id, userID)
	if err != nil {
		switch {
		case errors.Is(err, rentalservice.ErrUnauthorized):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case errors.Is(err, rentaldomain.ErrRentalNotActive), errors.Is(err, rentaldomain.ErrAlreadyConfirmed):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			shared.HandleServiceErr(c, err)
		}
		return
	}

	c.JSON(http.StatusOK, ToRentalResponse(rental))
}

// Dispute godoc
// @Summary      Reportar daño (disputa)
// @Description  El propietario reporta que la herramienta fue devuelta dañada. El estado pasa a "disputed" y MP captura el depósito como penalización. Solo el propietario puede llamar este endpoint
// @Tags         rentas
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path string           true "UUID de la renta"
// @Param        body body DisputeRequest true "Motivo de la disputa"
// @Success      200 {object} RentalResponse
// @Failure      400 {object} dto.ErrResponse
// @Failure      401 {object} dto.ErrResponse
// @Failure      403 {object} dto.ErrResponse "Solo el propietario puede reportar daño"
// @Failure      404 {object} dto.ErrResponse
// @Failure      409 {object} dto.ErrResponse "La renta no está activa"
// @Router       /rentals/{id}/dispute [post]
func (h *RentalHandler) Dispute(c *gin.Context) {
	id, err := shared.ParseUUID(c, "id")
	if err != nil {
		return
	}

	var req DisputeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ownerID := sharedmiddleware.UserIDFromContext(c)

	rental, err := h.rentalSvc.Dispute(c.Request.Context(), id, ownerID, req.Reason)
	if err != nil {
		switch {
		case errors.Is(err, rentalservice.ErrUnauthorized):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case errors.Is(err, rentaldomain.ErrRentalNotActive):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			shared.HandleServiceErr(c, err)
		}
		return
	}

	c.JSON(http.StatusOK, ToRentalResponse(rental))
}

// CancelRental godoc
// @Summary      Cancelar renta
// @Description  Cancela una renta. Libera la herramienta y cancela la pre-autorización en MP (devuelve fondos al solicitante)
// @Tags         rentas
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "UUID de la renta"
// @Success      200 {object} RentalResponse
// @Failure      401 {object} dto.ErrResponse
// @Failure      403 {object} dto.ErrResponse
// @Failure      404 {object} dto.ErrResponse
// @Failure      409 {object} dto.ErrResponse "No se puede cancelar una renta completada"
// @Router       /rentals/{id} [delete]
func (h *RentalHandler) CancelRental(c *gin.Context) {
	id, err := shared.ParseUUID(c, "id")
	if err != nil {
		return
	}

	userID := sharedmiddleware.UserIDFromContext(c)

	rental, err := h.rentalSvc.Cancel(c.Request.Context(), id, userID)
	if err != nil {
		switch {
		case errors.Is(err, rentalservice.ErrUnauthorized):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case errors.Is(err, rentaldomain.ErrCannotCancelDone):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			shared.HandleServiceErr(c, err)
		}
		return
	}

	c.JSON(http.StatusOK, ToRentalResponse(rental))
}

func (h *RentalHandler) GetMessages(c *gin.Context) {
	id, err := shared.ParseUUID(c, "id")
	if err != nil {
		return
	}
	userID := sharedmiddleware.UserIDFromContext(c)
	msgs, err := h.rentalSvc.GetMessages(c.Request.Context(), id, userID)
	if err != nil {
		if errors.Is(err, rentalservice.ErrUnauthorized) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		shared.HandleServiceErr(c, err)
		return
	}
	c.JSON(http.StatusOK, ToMessageListResponse(msgs))
}

func (h *RentalHandler) SendMessage(c *gin.Context) {
	id, err := shared.ParseUUID(c, "id")
	if err != nil {
		return
	}
	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID := sharedmiddleware.UserIDFromContext(c)
	msg, err := h.rentalSvc.SendMessage(c.Request.Context(), rentalports.SendMessageInput{
		RentalID: id,
		SenderID: userID,
		Message:  req.Message,
	})
	if err != nil {
		if errors.Is(err, rentalservice.ErrUnauthorized) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, ToMessageResponse(msg))
}
