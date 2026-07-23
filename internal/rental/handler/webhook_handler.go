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

	h.confirmPaymentByID(c.Request.Context(), notification.Data.ID)
	c.JSON(http.StatusOK, gin.H{"received": true})
}

// confirmPaymentByID consulta el pago en Mercado Pago y actualiza la renta (o
// la suscripción) que le corresponde según el external_reference. Es
// idempotente: se puede llamar tanto desde el webhook como desde el retorno
// del navegador (back_url) sin efectos dobles, porque la fuente de verdad es
// siempre el estado que MP reporta para ese payment_id.
func (h *WebhookHandler) confirmPaymentByID(ctx context.Context, paymentID string) {
	if h.paymentProvider == nil || paymentID == "" {
		return
	}

	info, err := h.paymentProvider.GetPaymentInfo(ctx, paymentID)
	if err != nil {
		log.Printf("WARN: no se pudo obtener info del pago %s: %v", paymentID, err)
		return
	}

	if info.ExternalRef == "" {
		return
	}

	if strings.HasPrefix(info.ExternalRef, userservice.SubscriptionExternalRefPrefix) {
		h.handleSubscriptionPayment(ctx, info)
		return
	}

	rentalID, err := uuid.Parse(info.ExternalRef)
	if err != nil {
		log.Printf("WARN: external_reference inválido: %s", info.ExternalRef)
		return
	}

	if err := h.rentalSvc.UpdatePaymentStatus(ctx, rentalID, info.ID, info.Status, info.PaymentTypeID); err != nil {
		log.Printf("WARN: no se pudo actualizar payment_status de renta %s: %v", rentalID, err)
	} else {
		log.Printf("MP confirm | renta %s actualizada → payment_id=%s status=%s", rentalID, info.ID, info.Status)
	}
}

// PaymentReturn atiende las back_urls de Checkout Pro (/payment/success, etc.).
// Además de mostrar la página de "puedes cerrar esta ventana", confirma el
// pago del lado del SERVIDOR usando el payment_id que MP agrega a la URL de
// retorno. Esto es clave: si el checkout de MP escapó al navegador externo
// (Safari) — porque abrió la app de MP o del banco vía deep link — el redirect
// de éxito ya no cae en el WebView de la app y esta nunca alcanza a llamar a
// confirm-payment. Confirmando aquí, el pago queda registrado igual, sin
// depender del webhook ni de que la app intercepte el redirect.
func (h *WebhookHandler) PaymentReturn(c *gin.Context) {
	status := c.Param("status")

	if status == "success" {
		paymentID := c.Query("payment_id")
		if paymentID == "" {
			paymentID = c.Query("collection_id")
		}
		if paymentID != "" {
			h.confirmPaymentByID(c.Request.Context(), paymentID)
		}
	}

	// Se devuelve una página HTML que rebota de vuelta a la app vía el deep
	// link toolshare://payment/<status>. Al reabrirse, la app reconcilia el
	// pago (onResume). Además queda un botón "Volver a la app" por si el
	// redirect automático no dispara.
	deepLink := "toolshare://payment/" + status

	var title, msg string
	switch status {
	case "success":
		title = "¡Pago confirmado!"
		msg = "Tu pago se registró correctamente. Regresando a la app…"
	case "pending":
		title = "Pago pendiente"
		msg = "Tu pago quedó pendiente de aprobación. Regresando a la app…"
	default:
		title = "Pago no completado"
		msg = "El pago no se completó. Regresando a la app…"
	}

	html := `<!DOCTYPE html><html lang="es"><head><meta charset="utf-8">` +
		`<meta name="viewport" content="width=device-width, initial-scale=1">` +
		`<title>` + title + `</title>` +
		`<script>function go(){window.location.href="` + deepLink + `";}` +
		`go();setTimeout(go,700);</script>` +
		`<style>body{font-family:-apple-system,Segoe UI,Roboto,sans-serif;background:#0f172a;` +
		`color:#e2e8f0;display:flex;min-height:100vh;align-items:center;justify-content:center;` +
		`margin:0;text-align:center}.c{padding:28px;max-width:360px}` +
		`.t{font-size:22px;font-weight:800;margin-bottom:10px}` +
		`.m{font-size:15px;color:#94a3b8;line-height:1.5;margin-bottom:24px}` +
		`a.btn{display:inline-block;background:#f97316;color:#fff;text-decoration:none;` +
		`font-weight:700;padding:14px 22px;border-radius:12px}</style></head>` +
		`<body><div class="c"><div class="t">` + title + `</div>` +
		`<div class="m">` + msg + `</div>` +
		`<a class="btn" href="` + deepLink + `">Volver a la app</a></div></body></html>`

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
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
