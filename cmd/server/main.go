// @title           Tool Rental API
// @version         1.0
// @description     API de renta de herramientas con aceptación mutua (apretón de manos digital) y pago con Mercado Pago.
// @description
// @description     ## Flujo de renta
// @description     1. Propietario registra herramienta con precio
// @description     2. Solicitante solicita renta → estado **pending** (fondos congelados en MP)
// @description     3. Ambos llaman `/confirm-delivery` con GPS → estado **active** + contrato SHA-256
// @description     4a. Ambos llaman `/confirm-return` → estado **completed** (MP cobra renta, devuelve depósito)
// @description     4b. Propietario llama `/dispute` → estado **disputed** (MP captura depósito como penalización)
//
// @contact.name   Angel Chame
// @contact.email  angelchame6@gmail.com
//
// @host            localhost:8080
// @BasePath        /api
//
// @securityDefinitions.apikey BearerAuth
// @in              header
// @name            Authorization
// @description     Pegar el token así: **Bearer &lt;token&gt;**
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/yourusername/tool-inventory-api/internal/adapters/primary/http/handler"
	"github.com/yourusername/tool-inventory-api/internal/adapters/primary/http/router"
	jwtadapter "github.com/yourusername/tool-inventory-api/internal/adapters/secondary/jwt"
	"github.com/yourusername/tool-inventory-api/internal/adapters/secondary/payment"
	"github.com/yourusername/tool-inventory-api/internal/adapters/secondary/postgres"
	"github.com/yourusername/tool-inventory-api/internal/adapters/secondary/storage"
	"github.com/yourusername/tool-inventory-api/internal/core/ports/output"
	"github.com/yourusername/tool-inventory-api/internal/core/service"
	"github.com/yourusername/tool-inventory-api/internal/infrastructure/database"

	_ "github.com/yourusername/tool-inventory-api/docs" // generado por swag init
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Archivo .env no encontrado, usando variables del sistema")
	}

	if err := database.Connect(); err != nil {
		log.Fatalf("No se pudo conectar a la base de datos: %v", err)
	}
	defer database.Close()

	// ── Adaptadores secundarios ────────────────────────────────────────────────
	tokenProvider := jwtadapter.NewProvider()

	uploadsDir := os.Getenv("UPLOADS_DIR")
	if uploadsDir == "" {
		uploadsDir = "./uploads"
	}
	uploadsBaseURL := os.Getenv("UPLOADS_BASE_URL")
	if uploadsBaseURL == "" {
		uploadsBaseURL = "http://localhost:8080/static"
	}
	fileStorage := storage.NewLocalStorage(uploadsDir, uploadsBaseURL)

	// Proveedor de pagos: real si MP_ACCESS_TOKEN está configurado, mock si no
	var paymentProvider output.PaymentProvider
	if mpToken := os.Getenv("MP_ACCESS_TOKEN"); mpToken != "" {
		paymentProvider = payment.NewMercadoPagoProvider(mpToken)
		log.Println("Mercado Pago: modo producción")
	} else {
		paymentProvider = payment.NewMockPaymentProvider()
		log.Println("Mercado Pago: modo mock (MP_ACCESS_TOKEN no configurado)")
	}

	userRepo := postgres.NewUserRepository(database.DB)
	toolRepo := postgres.NewToolRepository(database.DB)
	rentalRepo := postgres.NewRentalRepository(database.DB)

	// ── Servicios ──────────────────────────────────────────────────────────────
	authSvc := service.NewAuthService(userRepo, tokenProvider)
	toolSvc := service.NewToolService(toolRepo, fileStorage)
	rentalSvc := service.NewRentalService(rentalRepo, toolRepo, paymentProvider)

	// ── Adaptadores primarios (HTTP) ───────────────────────────────────────────
	handlers := router.Handlers{
		Auth:    handler.NewAuthHandler(authSvc),
		Tool:    handler.NewToolHandler(toolSvc),
		Rental:  handler.NewRentalHandler(rentalSvc),
		Webhook: handler.NewWebhookHandler(),
	}

	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := router.New(handlers, tokenProvider, uploadsDir)

	// Swagger UI en /docs/index.html
	r.GET("/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// ── Servidor con graceful shutdown ─────────────────────────────────────────
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("Servidor en http://localhost:%s", port)
		log.Printf("Swagger UI en http://localhost:%s/docs/index.html", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Error al iniciar el servidor: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Apagando el servidor...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Error durante el apagado: %v", err)
	}
	log.Println("Servidor apagado correctamente")
}
