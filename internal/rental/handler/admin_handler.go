package rentalhandler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	rentalports "github.com/yourusername/tool-inventory-api/internal/rental/ports"
)

type AdminHandler struct {
	svc rentalports.AdminService
}

func NewAdminHandler(svc rentalports.AdminService) *AdminHandler {
	return &AdminHandler{svc: svc}
}

// GetStats godoc
// @Summary Obtener estadísticas del dashboard de administrador
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Success 200 {object} rentalports.AdminStatsOutput
// @Router /admin/stats [get]
func (h *AdminHandler) GetStats(c *gin.Context) {
	stats, err := h.svc.GetStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// ListRentals godoc
// @Summary Listar todos los alquileres (con filtro opcional de estatus)
// @Tags admin
// @Produce json
// @Param status query string false "Filtrar por estatus (ej. disputed)"
// @Security BearerAuth
// @Success 200 {array} rentaldomain.Rental
// @Router /admin/rentals [get]
func (h *AdminHandler) ListRentals(c *gin.Context) {
	status := c.Query("status")
	rentals, err := h.svc.ListRentals(c.Request.Context(), status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ToRentalListResponse(rentals))
}

// ResolveDispute godoc
// @Summary Dictaminar arbitraje sobre un alquiler en disputa
// @Tags admin
// @Accept json
// @Produce json
// @Param id path string true "ID del Alquiler"
// @Param request body rentalports.ResolveDisputeInput true "Acción y notas de dictamen"
// @Security BearerAuth
// @Success 200 {object} RentalResponse
// @Router /admin/rentals/{id}/resolve [post]
func (h *AdminHandler) ResolveDispute(c *gin.Context) {
	idStr := c.Param("id")
	rentalID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de alquiler inválido"})
		return
	}

	var inp rentalports.ResolveDisputeInput
	if err := c.ShouldBindJSON(&inp); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	inp.RentalID = rentalID

	resolved, err := h.svc.ResolveDispute(c.Request.Context(), inp)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ToResolveDisputeResponse(resolved))
}

// GetInsuranceClaim godoc
// @Summary Consultar el monto de seguro y los datos bancarios del propietario de una renta
// @Description A diferencia de la respuesta de /resolve (que solo se ve una vez), este endpoint se puede llamar cuantas veces haga falta para volver a consultar los datos bancarios de una disputa ya resuelta a favor del propietario
// @Tags admin
// @Produce json
// @Param id path string true "ID del Alquiler"
// @Security BearerAuth
// @Success 200 {object} InsuranceClaimResponse
// @Router /admin/rentals/{id}/bank-account [get]
func (h *AdminHandler) GetInsuranceClaim(c *gin.Context) {
	idStr := c.Param("id")
	rentalID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de alquiler inválido"})
		return
	}

	claim, err := h.svc.GetInsuranceClaim(c.Request.Context(), rentalID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ToInsuranceClaimResponse(claim))
}
