package router

import (
	"net/http"
	"github.com/gin-gonic/gin"
	userhandler "github.com/yourusername/tool-inventory-api/internal/user/handler"
	toolhandler "github.com/yourusername/tool-inventory-api/internal/tool/handler"
	rentalhandler "github.com/yourusername/tool-inventory-api/internal/rental/handler"
	sharedmiddleware "github.com/yourusername/tool-inventory-api/internal/shared/middleware"
	userdomain "github.com/yourusername/tool-inventory-api/internal/user/domain"
	sharedports "github.com/yourusername/tool-inventory-api/internal/shared/ports"
)

type Handlers struct {
	Auth    *userhandler.AuthHandler
	Tool    *toolhandler.ToolHandler
	Rental  *rentalhandler.RentalHandler
	Webhook *rentalhandler.WebhookHandler
}

func New(h Handlers, tokenProvider sharedports.TokenProvider, uploadsDir string) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), corsMiddleware())

	// Fotos de herramientas
	r.Static("/static", uploadsDir)

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/favicon.ico", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
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
	protected.Use(sharedmiddleware.RequireAuth(tokenProvider))
	{
		// Propietario: gestión de herramientas
		owner := protected.Group("")
		owner.Use(sharedmiddleware.RequireRole(userdomain.RoleOwner))
		{
			owner.GET("/owner/tools", h.Tool.GetMyTools)
			owner.POST("/tools", h.Tool.CreateTool)
			owner.PUT("/tools/:id", h.Tool.UpdateTool)
			owner.DELETE("/tools/:id", h.Tool.DeleteTool)
			owner.POST("/tools/:id/photo", h.Tool.UploadPhoto)
			owner.POST("/tools/predict-condition", h.Tool.PredictCondition)
			owner.GET("/tools/auto-valuate", h.Tool.AutoValuate)
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
			rentals.GET("/:id/messages", h.Rental.GetMessages)
			rentals.POST("/:id/messages", h.Rental.SendMessage)
		}
	}

	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "ruta no encontrada"})
	})

	return r
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
