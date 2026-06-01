// Package auth contiene la lógica de generación y validación de tokens JWT.
package auth

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/yourusername/tool-inventory-api/internal/model"
)

// Claims define el payload que va dentro del token JWT.
// Incluye los campos estándar (RegisteredClaims) más los datos del usuario.
type Claims struct {
	UserID uuid.UUID  `json:"user_id"`
	Role   model.Role `json:"role"`
	jwt.RegisteredClaims
}

// GenerateToken crea un JWT firmado para el usuario dado.
// El token incluye el ID y el rol del usuario, y expira según JWT_EXPIRATION.
func GenerateToken(userID uuid.UUID, role model.Role) (string, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "", errors.New("JWT_SECRET no está configurado")
	}

	// Parsear duración desde variable de entorno (default: 24 horas)
	expStr := os.Getenv("JWT_EXPIRATION")
	if expStr == "" {
		expStr = "24h"
	}
	expDuration, err := time.ParseDuration(expStr)
	if err != nil {
		return "", fmt.Errorf("JWT_EXPIRATION inválido: %w", err)
	}

	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "tool-inventory-api",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ValidateToken verifica y parsea un token JWT.
// Retorna los Claims si el token es válido, o error si está expirado/inválido.
func ValidateToken(tokenStr string) (*Claims, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, errors.New("JWT_SECRET no está configurado")
	}

	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		// Verificar que el algoritmo de firma es el esperado (HS256)
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("algoritmo de firma inesperado: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("token inválido: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("claims del token inválidos")
	}

	return claims, nil
}
