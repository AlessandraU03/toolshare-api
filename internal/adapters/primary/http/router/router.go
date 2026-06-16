package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/tool-inventory-api/internal/adapters/primary/http/handler"
	"github.com/yourusername/tool-inventory-api/internal/adapters/primary/http/middleware"
	"github.com/yourusername/tool-inventory-api/internal/core/domain"
	"github.com/yourusername/tool-inventory-api/internal/core/ports/output"
)

type Handlers struct {
	Auth    *handler.AuthHandler
	Tool    *handler.ToolHandler
	Rental  *handler.RentalHandler
	Webhook *handler.WebhookHandler
}

func New(h Handlers, tokenProvider output.TokenProvider, uploadsDir string) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// Fotos de herramientas
	r.Static("/static", uploadsDir)

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := r.Group("/api")

	// ── Públicas ───────────────────────────────────────────────────────────────
	auth := api.Group("/auth")
	{
		auth.POST("/register", h.Auth.Register)
		auth.POST("/login", h.Auth.Login)
	}

	api.GET("/tools", h.Tool.GetTools)
	api.GET("/tools/:id", h.Tool.GetTool)
	api.GET("/pricing", h.Tool.GetPricingSuggestion)

	// Webhook de Mercado Pago (público, MP llama directamente)
	api.POST("/webhooks/mercadopago", h.Webhook.MercadoPago)

	// ── Autenticadas ───────────────────────────────────────────────────────────
	protected := api.Group("")
	protected.Use(middleware.RequireAuth(tokenProvider))
	{
		// Propietario: gestión de herramientas
		owner := protected.Group("")
		owner.Use(middleware.RequireRole(domain.RoleOwner))
		{
			owner.GET("/owner/tools", h.Tool.GetMyTools)
			owner.POST("/tools", h.Tool.CreateTool)
			owner.PUT("/tools/:id", h.Tool.UpdateTool)
			owner.DELETE("/tools/:id", h.Tool.DeleteTool)
			owner.POST("/tools/:id/photo", h.Tool.UploadPhoto)
		}

		// Rentas
		rentals := protected.Group("/rentals")
		{
			rentals.POST("", h.Rental.CreateRental)
			rentals.GET("", h.Rental.GetRentals)
			rentals.GET("/:id", h.Rental.GetRental)
			rentals.POST("/:id/confirm-delivery", h.Rental.ConfirmDelivery)
			rentals.POST("/:id/confirm-return", h.Rental.ConfirmReturn)
			rentals.POST("/:id/dispute", h.Rental.Dispute)
			rentals.DELETE("/:id", h.Rental.CancelRental)
		}
	}

	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "ruta no encontrada"})
	})

	return r
}
