package router

import (
	"github.com/gin-gonic/gin"
	rentalhandler "github.com/yourusername/tool-inventory-api/internal/rental/handler"
	reviewhandler "github.com/yourusername/tool-inventory-api/internal/review/handler"
	sharedmiddleware "github.com/yourusername/tool-inventory-api/internal/shared/middleware"
	sharedports "github.com/yourusername/tool-inventory-api/internal/shared/ports"
	toolhandler "github.com/yourusername/tool-inventory-api/internal/tool/handler"
	userdomain "github.com/yourusername/tool-inventory-api/internal/user/domain"
	userhandler "github.com/yourusername/tool-inventory-api/internal/user/handler"
	"net/http"
)

type Handlers struct {
	Auth    *userhandler.AuthHandler
	Tool    *toolhandler.ToolHandler
	Rental  *rentalhandler.RentalHandler
	Webhook *rentalhandler.WebhookHandler
	Admin   *rentalhandler.AdminHandler
	Review  *reviewhandler.ReviewHandler
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
		auth.POST("/verify-kyc", h.Auth.VerifyKyc)
	}

	api.GET("/tools", h.Tool.GetTools)
	api.GET("/tools/:id", h.Tool.GetTool)
	api.GET("/tools/:id/reviews", h.Review.GetToolReviews)
	api.GET("/pricing", h.Tool.GetPricingSuggestion)

	// Webhook de Mercado Pago (público, MP llama directamente)
	api.POST("/webhooks/mercadopago", h.Webhook.MercadoPago)

	// ── Autenticadas ───────────────────────────────────────────────────────────
	protected := api.Group("")
	protected.Use(sharedmiddleware.RequireAuth(tokenProvider))
	{
		protected.GET("/auth/me", h.Auth.Me)
		protected.POST("/auth/subscribe/preference", h.Auth.SubscribePreference)
		protected.POST("/auth/subscribe/confirm", h.Auth.ConfirmSubscription)

		protected.GET("/users/:id/reviews", h.Review.GetUserReviews)

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
			rentals.POST("/:id/preference", h.Rental.CreatePreference)
			rentals.POST("/:id/confirm-delivery", h.Rental.ConfirmDelivery)
			rentals.POST("/:id/confirm-return", h.Rental.ConfirmReturn)
			rentals.POST("/:id/dispute", h.Rental.Dispute)
			rentals.POST("/:id/review", h.Review.CreateReview)
			rentals.DELETE("/:id", h.Rental.CancelRental)
			rentals.GET("/:id/messages", h.Rental.GetMessages)
			rentals.POST("/:id/messages", h.Rental.SendMessage)
			rentals.GET("/:id/verify-contract", h.Rental.VerifyContract)
			rentals.GET("/:id/stream", h.Rental.StreamRental)
		}

		// Administrador: monitoreo y arbitraje
		admin := protected.Group("/admin")
		admin.Use(sharedmiddleware.RequireRole(userdomain.RoleAdmin))
		{
			admin.GET("/stats", h.Admin.GetStats)
			admin.GET("/rentals", h.Admin.ListRentals)
			admin.POST("/rentals/:id/resolve", h.Admin.ResolveDispute)
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
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-Device-ID")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
