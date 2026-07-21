package middleware

import (
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders agrega cabeceras de seguridad HTTP estándar a toda respuesta.
// Corrige el hallazgo H-09 (Nuclei/ZAP: cabeceras de seguridad faltantes).
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Permissions-Policy", "geolocation=(), camera=(), microphone=()")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		h.Set("X-Permitted-Cross-Domain-Policies", "none")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		// Solo tiene sentido en HTTPS; en Cloud Run el tráfico llega ya cifrado.
		h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		c.Next()
	}
}

// CORS restringe los orígenes permitidos a una lista blanca (ALLOWED_ORIGINS,
// separada por comas) en vez del comodín "*" combinado con credenciales,
// que es inválido según la especificación CORS. Corrige el hallazgo H-01.
func CORS() gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, o := range strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			allowed[o] = true
		}
	}
	// Fallback de desarrollo si no se configuró nada: solo localhost.
	if len(allowed) == 0 {
		allowed["http://localhost:8080"] = true
		allowed["http://localhost:8090"] = true
		allowed["http://127.0.0.1:8080"] = true
	}

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin != "" && allowed[origin] {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Set("Vary", "Origin")
		}
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-Device-ID")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// Recovery reemplaza a gin.Recovery(): captura panics y responde siempre con
// un JSON genérico, sin exponer el mensaje/stack interno al cliente.
// Corrige el hallazgo H-04 (ZAP: Application Error Disclosure).
func Recovery() gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, recovered any) {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "error interno del servidor"})
	})
}

// RateLimit limita el número de solicitudes por IP en una ventana fija de
// tiempo. Pensado para endpoints sensibles como login/register.
// Corrige el hallazgo H-02 (ausencia de rate limiting en autenticación).
func RateLimit(maxRequests int, window time.Duration) gin.HandlerFunc {
	type bucket struct {
		count     int
		resetAt   time.Time
	}

	var mu sync.Mutex
	buckets := map[string]*bucket{}

	return func(c *gin.Context) {
		ip := c.ClientIP()

		mu.Lock()
		b, ok := buckets[ip]
		now := time.Now()
		if !ok || now.After(b.resetAt) {
			b = &bucket{count: 0, resetAt: now.Add(window)}
			buckets[ip] = b
		}
		b.count++
		count := b.count
		mu.Unlock()

		if count > maxRequests {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "demasiados intentos, intenta de nuevo más tarde"})
			return
		}
		c.Next()
	}
}
