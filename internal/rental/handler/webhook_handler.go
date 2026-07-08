package rentalhandler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	rentalports "github.com/yourusername/tool-inventory-api/internal/rental/ports"
	sharedports "github.com/yourusername/tool-inventory-api/internal/shared/ports"
	userports "github.com/yourusername/tool-inventory-api/internal/user/ports"
	userservice "github.com/yourusername/tool-inventory-api/internal/user/service"
)

type WebhookHandler struct {
	rentalSvc       rentalports.RentalService
	userRepo        userports.UserRepository
	paymentProvider sharedports.PaymentProvider
	webhookSecret   string
}

func NewWebhookHandler(rentalSvc rentalports.RentalService, userRepo userports.UserRepository, paymentProvider sharedports.PaymentProvider, webhookSecret string) *WebhookHandler {
	return &WebhookHandler{rentalSvc: rentalSvc, userRepo: userRepo, paymentProvider: paymentProvider, webhookSecret: webhookSecret}
}

// verifySignature valida el header x-signature que envía Mercado Pago.
// Formato: "ts=<timestamp>,v1=<hmac_sha256_hex>". El manifest firmado es
// "id:<data.id>;request-id:<x-request-id>;ts:<ts>;" con data.id en minúsculas.
// Ver: https://www.mercadopago.com/developers/es/docs/checkout-api/additional-content/security/signature
func (h *WebhookHandler) verifySignature(c *gin.Context) bool {
	if h.webhookSecret == "" {
		return true // sin secreto configurado (dev/sandbox): no se valida
	}

	xSignature := c.GetHeader("x-signature")
	xRequestID := c.GetHeader("x-request-id")
	dataID := c.Query("data.id")
	if xSignature == "" || xRequestID == "" || dataID == "" {
		return false
	}

	var ts, v1 string
	for _, part := range strings.Split(xSignature, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch strings.TrimSpace(kv[0]) {
		case "ts":
			ts = strings.TrimSpace(kv[1])
		case "v1":
			v1 = strings.TrimSpace(kv[1])
		}
	}
	if ts == "" || v1 == "" {
		return false
	}

	manifest := "id:" + strings.ToLower(dataID) + ";request-id:" + xRequestID + ";ts:" + ts + ";"

	mac := hmac.New(sha256.New, []byte(h.webhookSecret))
	mac.Write([]byte(manifest))
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(v1))
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
	if !h.verifySignature(c) {
		log.Printf("WARN: firma de webhook MP inválida o ausente")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "firma inválida"})
		return
	}

	var notification mpNotification
	if err := c.ShouldBindJSON(&notification); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "notificación inválida"})
		return
	}

	log.Printf("MP Webhook | type=%s action=%s payment_id=%s",
		notification.Type, notification.Action, notification.Data.ID)

	// Solo procesar notificaciones de pago
	if notification.Type != "payment" || notification.Data.ID == "" {
		c.JSON(http.StatusOK, gin.H{"received": true})
		return
	}

	if h.paymentProvider == nil {
		c.JSON(http.StatusOK, gin.H{"received": true})
		return
	}

	ctx := c.Request.Context()

	info, err := h.paymentProvider.GetPaymentInfo(ctx, notification.Data.ID)
	if err != nil {
		log.Printf("WARN: no se pudo obtener info del pago %s: %v", notification.Data.ID, err)
		c.JSON(http.StatusOK, gin.H{"received": true})
		return
	}

	if info.ExternalRef == "" {
		c.JSON(http.StatusOK, gin.H{"received": true})
		return
	}

	if strings.HasPrefix(info.ExternalRef, userservice.SubscriptionExternalRefPrefix) {
		h.handleSubscriptionPayment(ctx, info)
		c.JSON(http.StatusOK, gin.H{"received": true})
		return
	}

	rentalID, err := uuid.Parse(info.ExternalRef)
	if err != nil {
		log.Printf("WARN: external_reference inválido: %s", info.ExternalRef)
		c.JSON(http.StatusOK, gin.H{"received": true})
		return
	}

	if err := h.rentalSvc.UpdatePaymentStatus(ctx, rentalID, info.ID, info.Status); err != nil {
		log.Printf("WARN: no se pudo actualizar payment_status de renta %s: %v", rentalID, err)
	} else {
		log.Printf("MP Webhook | renta %s actualizada → payment_id=%s status=%s", rentalID, info.ID, info.Status)
	}

	c.JSON(http.StatusOK, gin.H{"received": true})
}

// handleSubscriptionPayment activa el plan Pro del usuario cuando MP confirma
// un pago aprobado de la preferencia de suscripción (external_reference "sub:<user_id>").
func (h *WebhookHandler) handleSubscriptionPayment(ctx context.Context, info sharedports.PaymentInfo) {
	if info.Status != "approved" {
		log.Printf("MP Webhook | suscripción %s con estado %s, no se activa Pro", info.ExternalRef, info.Status)
		return
	}

	userIDStr := strings.TrimPrefix(info.ExternalRef, userservice.SubscriptionExternalRefPrefix)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		log.Printf("WARN: external_reference de suscripción inválido: %s", info.ExternalRef)
		return
	}

	if err := h.userRepo.UpdateIsPro(ctx, userID, true); err != nil {
		log.Printf("WARN: no se pudo activar plan Pro del usuario %s: %v", userID, err)
		return
	}

	log.Printf("MP Webhook | usuario %s activó plan Pro → payment_id=%s", userID, info.ID)
}
