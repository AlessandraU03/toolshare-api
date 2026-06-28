// @title           Tool Rental API
// @version         1.0
// @description     API de renta de herramientas con aceptación mutua (apretón de manos digital) y pago con Mercado Pago.
// @contact.name   Angel Chame
// @contact.email  angelchame6@gmail.com
// @BasePath        /api
// @securityDefinitions.apikey BearerAuth
// @in              header
// @name            Authorization
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

	userhandler "github.com/yourusername/tool-inventory-api/internal/user/handler"
	toolhandler "github.com/yourusername/tool-inventory-api/internal/tool/handler"
	rentalhandler "github.com/yourusername/tool-inventory-api/internal/rental/handler"
	"github.com/yourusername/tool-inventory-api/internal/shared/router"
	jwtadapter "github.com/yourusername/tool-inventory-api/internal/shared/jwt"
	"github.com/yourusername/tool-inventory-api/internal/shared/payment"
	userpostgres "github.com/yourusername/tool-inventory-api/internal/user/postgres"
	toolpostgres "github.com/yourusername/tool-inventory-api/internal/tool/postgres"
	rentalpostgres "github.com/yourusername/tool-inventory-api/internal/rental/postgres"
	"github.com/yourusername/tool-inventory-api/internal/shared/storage"
	"github.com/yourusername/tool-inventory-api/internal/shared/ports"
	userservice "github.com/yourusername/tool-inventory-api/internal/user/service"
	userports "github.com/yourusername/tool-inventory-api/internal/user/ports"
	userdomain "github.com/yourusername/tool-inventory-api/internal/user/domain"
	toolservice "github.com/yourusername/tool-inventory-api/internal/tool/service"
	rentalservice "github.com/yourusername/tool-inventory-api/internal/rental/service"
	"github.com/yourusername/tool-inventory-api/internal/shared/database"

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
	var paymentProvider sharedports.PaymentProvider
	if mpToken := os.Getenv("MP_ACCESS_TOKEN"); mpToken != "" {
		paymentProvider = payment.NewMercadoPagoProvider(mpToken)
		log.Println("Mercado Pago: modo producción")
	} else {
		paymentProvider = payment.NewMockPaymentProvider()
		log.Println("Mercado Pago: modo mock (MP_ACCESS_TOKEN no configurado)")
	}

	userRepo := userpostgres.NewUserRepository(database.DB)
	toolRepo := toolpostgres.NewToolRepository(database.DB)
	rentalRepo := rentalpostgres.NewRentalRepository(database.DB)

	// ── Servicios ──────────────────────────────────────────────────────────────
	authSvc := userservice.NewAuthService(userRepo, tokenProvider)
	toolSvc := toolservice.NewToolService(toolRepo, userRepo, fileStorage)
	rentalSvc := rentalservice.NewRentalService(rentalRepo, toolRepo, paymentProvider)
	adminSvc := rentalservice.NewAdminService(rentalRepo, toolRepo, paymentProvider)

	// Sembrar usuario admin si no existe
	_, _ = authSvc.Register(context.Background(), userports.RegisterInput{
		Name:     "Admin ToolShare",
		Email:    "admin@toolshare.com",
		Password: "admin123",
		Role:     userdomain.RoleAdmin,
		Phone:    "9610000000",
		INE:      "ADMIN000000000000",
	})

	// ── Adaptadores primarios (HTTP) ───────────────────────────────────────────
	handlers := router.Handlers{
		Auth:    userhandler.NewAuthHandler(authSvc),
		Tool:    toolhandler.NewToolHandler(toolSvc),
		Rental:  rentalhandler.NewRentalHandler(rentalSvc),
		Webhook: rentalhandler.NewWebhookHandler(),
		Admin:   rentalhandler.NewAdminHandler(adminSvc),
	}

	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := router.New(handlers, tokenProvider, uploadsDir)

	// Swagger UI en /docs/index.html
	r.GET("/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// ── Servidor con graceful shutdown ─────────────────────────────────────────
	port := os.Getenv("PORT")
	if port == "" {
		port = os.Getenv("SERVER_PORT")
		if port == "" {
			port = "8080"
		}
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
