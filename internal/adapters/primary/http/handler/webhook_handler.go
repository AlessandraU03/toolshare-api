package handler

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type WebhookHandler struct{}

func NewWebhookHandler() *WebhookHandler {
	return &WebhookHandler{}
}

type mpNotification struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Action string `json:"action"`
	Data   struct {
		ID string `json:"id"`
	} `json:"data"`
}

// MercadoPago godoc
// @Summary      Webhook de Mercado Pago
// @Description  Recibe notificaciones de cambio de estado de pagos. Mercado Pago llama este endpoint automáticamente
// @Tags         webhooks
// @Accept       json
// @Produce      json
// @Param        body body mpNotification false "Notificación de MP"
// @Success      200 {object} dto.MsgResponse
// @Router       /webhooks/mercadopago [post]
func (h *WebhookHandler) MercadoPago(c *gin.Context) {
	var notification mpNotification
	if err := c.ShouldBindJSON(&notification); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "notificación inválida"})
		return
	}

	log.Printf("MP Webhook | type=%s action=%s payment_id=%s",
		notification.Type, notification.Action, notification.Data.ID)

	// TODO: actualizar payment_status en la BD cuando MP confirme el cambio de estado
	// Consultar /v1/payments/{id} y actualizar rental.payment_status

	c.JSON(http.StatusOK, gin.H{"received": true})
}
