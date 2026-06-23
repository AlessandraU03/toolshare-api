package jwtadapter

import (
	"errors"
	"fmt"
	"os"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	sharedports "github.com/yourusername/tool-inventory-api/internal/shared/ports"
	userdomain "github.com/yourusername/tool-inventory-api/internal/user/domain"
)

type claims struct {
	UserID uuid.UUID       `json:"user_id"`
	Role   userdomain.Role `json:"role"`
	gojwt.RegisteredClaims
}

type Provider struct{}

func NewProvider() sharedports.TokenProvider {
	return &Provider{}
}

func (p *Provider) Generate(userID uuid.UUID, role userdomain.Role) (string, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "", errors.New("JWT_SECRET no está configurado")
	}

	expStr := os.Getenv("JWT_EXPIRATION")
	if expStr == "" {
		expStr = "24h"
	}
	exp, err := time.ParseDuration(expStr)
	if err != nil {
		return "", fmt.Errorf("JWT_EXPIRATION inválido: %w", err)
	}

	c := claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: gojwt.RegisteredClaims{
			ExpiresAt: gojwt.NewNumericDate(time.Now().Add(exp)),
			IssuedAt:  gojwt.NewNumericDate(time.Now()),
			Issuer:    "tool-rental-api",
		},
	}

	token := gojwt.NewWithClaims(gojwt.SigningMethodHS256, c)
	return token.SignedString([]byte(secret))
}

func (p *Provider) Validate(tokenStr string) (uuid.UUID, userdomain.Role, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return uuid.Nil, "", errors.New("JWT_SECRET no está configurado")
	}

	token, err := gojwt.ParseWithClaims(tokenStr, &claims{}, func(t *gojwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*gojwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("algoritmo inesperado: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("token inválido: %w", err)
	}

	c, ok := token.Claims.(*claims)
	if !ok || !token.Valid {
		return uuid.Nil, "", errors.New("claims del token inválidos")
	}

	return c.UserID, c.Role, nil
}
