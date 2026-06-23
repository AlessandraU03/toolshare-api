package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var DB *pgxpool.Pool

func Connect() error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL no está definida")
	}

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("error al parsear DATABASE_URL: %w", err)
	}

	config.MaxConns = 25
	config.MinConns = 5
	config.MaxConnLifetime = 1 * time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return fmt.Errorf("error al crear pool de conexiones: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("no se pudo conectar a PostgreSQL: %w", err)
	}

	DB = pool
	log.Println("Conexión a PostgreSQL establecida")
	return nil
}

func Close() {
	if DB != nil {
		DB.Close()
		log.Println("Pool de conexiones cerrado")
	}
}
