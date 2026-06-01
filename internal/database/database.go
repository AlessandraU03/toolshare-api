// Package database gestiona la conexión y el pool de conexiones a PostgreSQL
// usando el driver pgx de alto rendimiento.
package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB es el pool global de conexiones a PostgreSQL.
// Usar un pool (pgxpool) es la práctica recomendada en aplicaciones concurrentes
// ya que reutiliza conexiones en lugar de abrir una nueva por cada request.
var DB *pgxpool.Pool

// Connect inicializa el pool de conexiones a PostgreSQL.
// Lee la URL de conexión desde la variable de entorno DATABASE_URL.
// Retorna error si la conexión falla, lo que detiene el inicio del servidor.
func Connect() error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL no está definida en las variables de entorno")
	}

	// Configurar el pool con parámetros razonables para una API REST
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("error al parsear DATABASE_URL: %w", err)
	}

	config.MaxConns = 25                     // Máximo de conexiones simultáneas
	config.MinConns = 5                      // Conexiones mínimas siempre abiertas
	config.MaxConnLifetime = 1 * time.Hour   // Recicla conexiones cada hora
	config.MaxConnIdleTime = 30 * time.Minute // Cierra conexiones ociosas tras 30min

	// Contexto con timeout para el intento inicial de conexión
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return fmt.Errorf("error al crear el pool de conexiones: %w", err)
	}

	// Verificar que la base de datos responde con un ping
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("no se pudo conectar a PostgreSQL: %w", err)
	}

	DB = pool
	log.Println("✅ Conexión a PostgreSQL establecida correctamente")
	return nil
}

// Close cierra el pool de conexiones de forma ordenada.
// Debe llamarse al apagar el servidor (defer database.Close()).
func Close() {
	if DB != nil {
		DB.Close()
		log.Println("🔌 Pool de conexiones a PostgreSQL cerrado")
	}
}
