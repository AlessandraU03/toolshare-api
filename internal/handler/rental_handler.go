package handler

import (
	"crypto/sha256"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yourusername/tool-inventory-api/internal/middleware"
	"github.com/yourusername/tool-inventory-api/internal/model"
	"github.com/yourusername/tool-inventory-api/internal/repository"
)

// RentalHandler agrupa los handlers del flujo de renta (checkout).
type RentalHandler struct {
	rentalRepo *repository.RentalRepository
	toolRepo   *repository.ToolRepository
}

// NewRentalHandler crea una nueva instancia del handler de rentas.
func NewRentalHandler(rentalRepo *repository.RentalRepository, toolRepo *repository.ToolRepository) *RentalHandler {
	return &RentalHandler{
		rentalRepo: rentalRepo,
		toolRepo:   toolRepo,
	}
}

// CreateRental godoc
// @Summary      Crear orden de renta
// @Description  Inicia el flujo de renta: crea la orden y devuelve la URL de pago de Mercado Pago.
// @Tags         rentals
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body model.CreateRentalRequest true "Datos de la renta"
// @Success      201 {object} model.CreateRentalResponse
// @Failure      400 {object} model.ErrorResponse
// @Failure      404 {object} model.ErrorResponse
// @Failure      409 {object} model.ErrorResponse
// @Failure      500 {object} model.ErrorResponse
// @Router       /api/rentals [post]
func (h *RentalHandler) CreateRental(c *gin.Context) {
	var req model.CreateRentalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error: formatValidationError(err),
		})
		return
	}

	requesterID := c.MustGet(middleware.ContextUserID).(uuid.UUID)

	// Verificar que la herramienta existe y está disponible
	tool, err := h.toolRepo.FindByID(c.Request.Context(), req.ToolID)
	if err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{
			Error: "herramienta no encontrada",
		})
		return
	}
	if !tool.IsAvailable {
		c.JSON(http.StatusConflict, model.ErrorResponse{
			Error: "la herramienta no está disponible en este momento",
		})
		return
	}

	// Generar URL de pago (por ahora simulada; aquí integrarías el SDK de Mercado Pago)
	// En producción: llamar a MP API con capture_mode: "manual" para retención de fondos
	expiresAt := time.Now().UTC().Add(30 * time.Minute)
	paymentURL := buildMercadoPagoURL(req.ToolID, req.Total)

	rental := &model.Rental{
		ToolID:      req.ToolID,
		RequesterID: requesterID,
		Days:        req.Days,
		PricePerDay: req.PricePerDay,
		Total:       req.Total,
		Deposit:     req.Deposit,
		Status:      model.RentalStatusPendingPayment,
		PaymentURL:  &paymentURL,
		ExpiresAt:   &expiresAt,
	}

	created, err := h.rentalRepo.Create(c.Request.Context(), rental)
	if err != nil {
		log.Printf("error al crear renta: %v", err)
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{
			Error: "error al crear la orden de renta",
		})
		return
	}

	c.JSON(http.StatusCreated, model.CreateRentalResponse{
		RentalID:   created.ID,
		Status:     created.Status,
		PaymentURL: paymentURL,
		ExpiresAt:  expiresAt,
	})
}

// ConfirmPayment godoc
// @Summary      Confirmar pago / retener fondos
// @Description  Registra el ID de pago de Mercado Pago y actualiza el estado a funds_held.
// @Tags         rentals
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        rental_id path string                    true "UUID de la renta"
// @Param        body      body model.ConfirmPaymentRequest true "ID de pago MP"
// @Success      200 {object} model.ConfirmPaymentResponse
// @Failure      400 {object} model.ErrorResponse
// @Failure      404 {object} model.ErrorResponse
// @Failure      500 {object} model.ErrorResponse
// @Router       /api/rentals/{rental_id}/payment [post]
func (h *RentalHandler) ConfirmPayment(c *gin.Context) {
	rentalID, err := parseUUID(c, "rental_id")
	if err != nil {
		return
	}

	var req model.ConfirmPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error: formatValidationError(err),
		})
		return
	}

	updated, err := h.rentalRepo.ConfirmPayment(c.Request.Context(), rentalID, req.PaymentID)
	if err != nil {
		log.Printf("error al confirmar pago: %v", err)
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{
			Error: "error al confirmar el pago",
		})
		return
	}

	c.JSON(http.StatusOK, model.ConfirmPaymentResponse{
		RentalID: updated.ID,
		Status:   updated.Status,
		Message:  "Fondos retenidos correctamente",
	})
}

// ConfirmDelivery godoc
// @Summary      Confirmar entrega física
// @Description  El solicitante o el propietario confirma que la herramienta fue entregada (con coordenadas GPS).
//
//	Cuando ambos confirman, el estado cambia a "delivered" y se genera el hash del contrato.
//
// @Tags         rentals
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        rental_id path string                       true "UUID de la renta"
// @Param        body      body model.ConfirmDeliveryRequest true "Coordenadas y rol"
// @Success      200 {object} model.ConfirmDeliveryResponse
// @Failure      400 {object} model.ErrorResponse
// @Failure      500 {object} model.ErrorResponse
// @Router       /api/rentals/{rental_id}/confirm-delivery [post]
func (h *RentalHandler) ConfirmDelivery(c *gin.Context) {
	rentalID, err := parseUUID(c, "rental_id")
	if err != nil {
		return
	}

	var req model.ConfirmDeliveryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error: formatValidationError(err),
		})
		return
	}

	// Generar hash del contrato digital (se usa al confirmar ambos lados)
	contractHash := buildContractHash(rentalID, req.Latitude, req.Longitude)

	updated, err := h.rentalRepo.ConfirmDelivery(
		c.Request.Context(),
		rentalID,
		req.Role,
		req.Latitude,
		req.Longitude,
		contractHash,
	)
	if err != nil {
		log.Printf("error al confirmar entrega: %v", err)
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{
			Error: "error al confirmar la entrega",
		})
		return
	}

	resp := model.ConfirmDeliveryResponse{
		RentalID:            updated.ID,
		Status:              updated.Status,
		DeliveryConfirmedAt: updated.DeliveryConfirmedAt,
	}
	if updated.ContractHash != nil {
		resp.ContractHash = *updated.ContractHash
	}

	c.JSON(http.StatusOK, resp)
}

// ProcessReturn godoc
// @Summary      Aceptar o rechazar devolución
// @Description  Solo el propietario puede aceptar o rechazar la devolución de la herramienta.
//
//	Si acepta → fondos liberados → estado "completed".
//	Si rechaza → disputa → estado "dispute".
//
// @Tags         rentals
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        rental_id path string              true "UUID de la renta"
// @Param        body      body model.ReturnRequest true "Aceptar o rechazar"
// @Success      200 {object} model.ReturnResponse
// @Failure      400 {object} model.ErrorResponse
// @Failure      500 {object} model.ErrorResponse
// @Router       /api/rentals/{rental_id}/return [post]
func (h *RentalHandler) ProcessReturn(c *gin.Context) {
	rentalID, err := parseUUID(c, "rental_id")
	if err != nil {
		return
	}

	var req model.ReturnRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error: formatValidationError(err),
		})
		return
	}

	updated, err := h.rentalRepo.ProcessReturn(c.Request.Context(), rentalID, req.Accepted, req.Reason)
	if err != nil {
		log.Printf("error al procesar devolución: %v", err)
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{
			Error: "error al procesar la devolución",
		})
		return
	}

	resp := model.ReturnResponse{
		RentalID: updated.ID,
		Status:   updated.Status,
	}

	if req.Accepted {
		resp.FundsReleased = true
		resp.DepositReturned = true
		// TODO: llamar a MP API para liberar los fondos retenidos
	} else {
		resp.FundsReleased = false
		resp.ArbitrationAlert = true
		// TODO: enviar alerta al equipo de arbitraje
		log.Printf("⚠️  DISPUTA abierta en renta %s: %s", rentalID, req.Reason)
	}

	c.JSON(http.StatusOK, resp)
}

// =============================================================================
// Helpers privados
// =============================================================================

// buildMercadoPagoURL construye la URL de pago de Mercado Pago.
// En producción esto debe hacer una llamada real a la API de MP.
func buildMercadoPagoURL(toolID uuid.UUID, total float64) string {
	mpBaseURL := os.Getenv("MERCADO_PAGO_CHECKOUT_URL")
	if mpBaseURL == "" {
		// URL simulada para desarrollo/MVP
		return fmt.Sprintf("https://www.mercadopago.com.mx/checkout/v1/redirect?pref_id=MOCK-%s", toolID.String())
	}
	return fmt.Sprintf("%s?tool_id=%s&amount=%.2f", mpBaseURL, toolID.String(), total)
}

// buildContractHash genera el hash SHA-256 del "contrato digital" de entrega.
func buildContractHash(rentalID uuid.UUID, lat, lng float64) string {
	data := fmt.Sprintf("%s|%.7f|%.7f|%d", rentalID.String(), lat, lng, time.Now().Unix())
	h := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%X", h)
}
