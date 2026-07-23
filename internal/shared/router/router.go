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
	"os"
)

type Handlers struct {
	Auth    *userhandler.AuthHandler
	Tool    *toolhandler.ToolHandler
	Rental  *rentalhandler.RentalHandler
	Webhook *rentalhandler.WebhookHandler
	Admin   *rentalhandler.AdminHandler
	Review  *reviewhandler.ReviewHandler
}

func New(h Handlers, tokenProvider sharedports.TokenProvider, uploadsDir string, mockPaymentMode bool) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), corsMiddleware())

	// Fotos de herramientas
	r.Static("/static", uploadsDir)

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Destino de las back_urls de Mercado Pago (Checkout Pro exige una URL
	// absoluta http(s), no un esquema de app como "toolshare://"). En la app,
	// el WebView intercepta la navegación hacia aquí antes de que complete;
	// esta ruta es solo un respaldo por si el redirect automático de MP la
	// alcanza a cargar primero.
	r.GET("/payment/:status", h.Webhook.PaymentReturn)

	// Config pública y segura de exponer al cliente (nunca la clave secreta de MP).
	r.GET("/api/config", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"mp_public_key": os.Getenv("MP_PUBLIC_KEY")})
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
		auth.GET("/verify-kyc/:job_id", h.Auth.GetKycJob)
		// Callback de OAuth de Mercado Pago: lo invoca MP redirigiendo el
		// navegador del propietario, sin header de autenticación.
		auth.GET("/mp-connect/callback", h.Auth.MPConnectCallback)
	}

	api.GET("/tools", h.Tool.GetTools)
	api.GET("/tools/:id", h.Tool.GetTool)
	api.GET("/tools/:id/reviews", h.Review.GetToolReviews)
	api.GET("/pricing", h.Tool.GetPricingSuggestion)

	// Webhook de Mercado Pago (público, MP llama directamente)
	api.POST("/webhooks/mercadopago", h.Webhook.MercadoPago)

	// Checkout simulado (solo en modo mock / desarrollo local): aprueba el
	// pago al instante y redirige de vuelta a la app. No existe en producción.
	if mockPaymentMode {
		api.GET("/mock/mp-checkout", h.Rental.MockCheckout)
	}

	// ── Autenticadas ───────────────────────────────────────────────────────────
	protected := api.Group("")
	protected.Use(sharedmiddleware.RequireAuth(tokenProvider))
	{
		protected.GET("/auth/me", h.Auth.Me)
		protected.POST("/auth/subscribe/preference", h.Auth.SubscribePreference)
		protected.POST("/auth/subscribe/confirm", h.Auth.ConfirmSubscription)
		protected.POST("/auth/cards", h.Auth.AddCard)
		protected.GET("/auth/cards", h.Auth.ListCards)
		protected.DELETE("/auth/cards/:id", h.Auth.DeleteCard)
		protected.GET("/auth/mp-connect/start", h.Auth.StartMPConnect)
		protected.GET("/auth/mp-connect/status", h.Auth.GetMPConnectStatus)

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
			owner.POST("/tools/extract-ticket-price", h.Tool.ExtractTicketPrice)
			owner.GET("/tools/extract-ticket-price/:job_id", h.Tool.GetTicketPriceJob)
			owner.GET("/tools/auto-valuate", h.Tool.AutoValuate)
			owner.POST("/tools/:id/insurance/preference", h.Tool.CreateInsurancePreference)
			owner.POST("/tools/:id/insurance/confirm", h.Tool.ConfirmInsurancePayment)
			owner.POST("/tools/:id/insurance/cancel", h.Tool.CancelInsurance)
		}

		// Rentas
		rentals := protected.Group("/rentals")
		{
			rentals.POST("", h.Rental.CreateRental)
			rentals.GET("", h.Rental.GetRentals)
			rentals.GET("/:id", h.Rental.GetRental)
			rentals.POST("/:id/preference", h.Rental.CreatePreference)
			rentals.POST("/:id/confirm-payment", h.Rental.ConfirmPayment)
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
