// Package main es el punto de entrada de la API RESTful de inventario de herramientas.
// Inicializa la base de datos, registra las rutas y arranca el servidor HTTP.
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
	"github.com/yourusername/tool-inventory-api/internal/database"
	"github.com/yourusername/tool-inventory-api/internal/handler"
	"github.com/yourusername/tool-inventory-api/internal/middleware"
	"github.com/yourusername/tool-inventory-api/internal/model"
	"github.com/yourusername/tool-inventory-api/internal/repository"
)

func main() {
	// -----------------------------------------------------------------
	// 1. Cargar variables de entorno desde el archivo .env
	//    En producción (Docker/Kubernetes) estas vienen del entorno real.
	// -----------------------------------------------------------------
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  Archivo .env no encontrado, usando variables del sistema")
	}

	// -----------------------------------------------------------------
	// 2. Conectar a PostgreSQL
	// -----------------------------------------------------------------
	if err := database.Connect(); err != nil {
		log.Fatalf("❌ No se pudo conectar a la base de datos: %v", err)
	}
	defer database.Close()

	// -----------------------------------------------------------------
	// 3. Instanciar repositorios y handlers (inyección de dependencias manual)
	// -----------------------------------------------------------------
	userRepo   := repository.NewUserRepository(database.DB)
	toolRepo   := repository.NewToolRepository(database.DB)
	rentalRepo := repository.NewRentalRepository(database.DB)

	authHandler   := handler.NewAuthHandler(userRepo)
	toolHandler   := handler.NewToolHandler(toolRepo)
	rentalHandler := handler.NewRentalHandler(rentalRepo, toolRepo)

	// -----------------------------------------------------------------
	// 4. Configurar el router Gin
	// -----------------------------------------------------------------
	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Middlewares globales
	router.Use(gin.Logger())   // Log de cada request
	router.Use(gin.Recovery()) // Recuperar de panics sin caer el servidor

	// Servir archivos estáticos de uploads (fotos de herramientas)
	router.Static("/uploads", "./uploads")

	// Endpoint de salud — útil para health checks en Docker/K8s
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "timestamp": time.Now().UTC()})
	})

	// -----------------------------------------------------------------
	// 5. Registrar rutas
	// -----------------------------------------------------------------
	api := router.Group("/api")
	{
		// --- Autenticación (públicas) ---
		authGroup := api.Group("/auth")
		{
			authGroup.POST("/register", authHandler.Register)
			authGroup.POST("/login", authHandler.Login)
		}

		// --- Herramientas ---
		toolsGroup := api.Group("/tools")
		{
			// GET /api/tools — Público: cualquiera puede ver el catálogo
			toolsGroup.GET("", toolHandler.GetTools)

			// GET /api/tools/suggest-price — Requiere autenticación
			toolsGroup.GET("/suggest-price",
				middleware.RequireAuth(),
				toolHandler.SuggestPrice,
			)

			// Rutas protegidas: requieren JWT válido + rol "owner"
			ownerRoutes := toolsGroup.Group("")
			ownerRoutes.Use(
				middleware.RequireAuth(),
				middleware.RequireRole(model.RoleOwner),
			)
			{
				// GET /api/tools/mine — solo las herramientas del propietario autenticado
				// ⚠️ Debe registrarse ANTES de /:id para evitar que Gin lo interprete como UUID
				ownerRoutes.GET("/mine", toolHandler.GetMyTools)

				ownerRoutes.POST("", toolHandler.CreateTool)
				ownerRoutes.PUT("/:id", toolHandler.UpdateTool)
				ownerRoutes.DELETE("/:id", toolHandler.DeleteTool)
				ownerRoutes.POST("/:id/photo", toolHandler.UploadPhoto)
			}
		}

		// --- Rentas (checkout flow) ---
		// Todas las rutas de rentas requieren JWT válido
		rentalsGroup := api.Group("/rentals")
		rentalsGroup.Use(middleware.RequireAuth())
		{
			// POST /api/rentals — Crear orden de renta (solo solicitantes)
			rentalsGroup.POST("", rentalHandler.CreateRental)

			// POST /api/rentals/:rental_id/payment — Confirmar pago / retener fondos
			rentalsGroup.POST("/:rental_id/payment", rentalHandler.ConfirmPayment)

			// POST /api/rentals/:rental_id/confirm-delivery — Confirmar entrega (GPS)
			rentalsGroup.POST("/:rental_id/confirm-delivery", rentalHandler.ConfirmDelivery)

			// POST /api/rentals/:rental_id/return — Aceptar/rechazar devolución (propietario)
			rentalsGroup.POST("/:rental_id/return", rentalHandler.ProcessReturn)
		}
	}

	// Ruta 404 para endpoints no definidos
	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, model.ErrorResponse{
			Error: "ruta no encontrada",
		})
	})

	// -----------------------------------------------------------------
	// 6. Arrancar el servidor HTTP con graceful shutdown
	// -----------------------------------------------------------------
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:         "0.0.0.0:" + port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("🚀 Servidor escuchando en http://0.0.0.0:%s", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("❌ Error al iniciar el servidor: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("⏳ Apagando el servidor...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("❌ Error durante el apagado: %v", err)
	}

	log.Println("✅ Servidor apagado correctamente")
}
