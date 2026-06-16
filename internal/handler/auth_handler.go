// Package handler contiene los controladores HTTP de la API.
// Cada handler recibe el request, llama al repositorio y devuelve la respuesta.
package handler

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/tool-inventory-api/internal/auth"
	"github.com/yourusername/tool-inventory-api/internal/model"
	"github.com/yourusername/tool-inventory-api/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

// AuthHandler agrupa los handlers relacionados con autenticación.
type AuthHandler struct {
	userRepo *repository.UserRepository
}

// NewAuthHandler crea una nueva instancia del handler de autenticación.
func NewAuthHandler(userRepo *repository.UserRepository) *AuthHandler {
	return &AuthHandler{userRepo: userRepo}
}

// Register godoc
// @Summary      Registrar nuevo usuario
// @Description  Crea una cuenta de Propietario o Solicitante
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body model.RegisterRequest true "Datos de registro"
// @Success      201  {object} model.AuthResponse
// @Failure      400  {object} model.ErrorResponse
// @Failure      409  {object} model.ErrorResponse
// @Failure      500  {object} model.ErrorResponse
// @Router       /api/auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req model.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error: formatValidationError(err),
		})
		return
	}

	// Normalizar email a minúsculas para evitar duplicados por capitalización
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	// Verificar si el email ya está registrado
	_, err := h.userRepo.FindByEmail(c.Request.Context(), req.Email)
	if err == nil {
		// FindByEmail tuvo éxito → el email ya existe
		c.JSON(http.StatusConflict, model.ErrorResponse{
			Error: "el email ya está registrado",
		})
		return
	}
	if err != repository.ErrNotFound {
		log.Printf("error al verificar email: %v", err)
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{
			Error: "error interno del servidor",
		})
		return
	}

	// Hashear la contraseña con bcrypt (cost=12 es un buen balance seguridad/velocidad)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		log.Printf("error al hashear contraseña: %v", err)
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{
			Error: "error interno del servidor",
		})
		return
	}

	// Convertir strings opcionales a punteros (solo si no están vacíos)
	var phone, ineNumber *string
	if req.Phone != "" {
		p := req.Phone
		phone = &p
	}
	if req.IneNumber != "" {
		i := req.IneNumber
		ineNumber = &i
	}

	user := &model.User{
		Name:      req.Name,
		Email:     req.Email,
		Password:  string(hashedPassword),
		Role:      req.Role,
		Phone:     phone,
		IneNumber: ineNumber,
	}

	created, err := h.userRepo.Create(c.Request.Context(), user)
	if err != nil {
		log.Printf("error al crear usuario: %v", err)
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{
			Error: "error interno del servidor",
		})
		return
	}

	// Generar JWT para el usuario recién registrado (auto-login)
	token, err := auth.GenerateToken(created.ID, created.Role)
	if err != nil {
		log.Printf("error al generar JWT: %v", err)
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{
			Error: "error al generar el token de acceso",
		})
		return
	}

	c.JSON(http.StatusCreated, model.AuthResponse{
		Token: token,
		User:  *created,
	})
}

// Login godoc
// @Summary      Iniciar sesión
// @Description  Autentica al usuario y devuelve un JWT
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body model.LoginRequest true "Credenciales"
// @Success      200  {object} model.AuthResponse
// @Failure      400  {object} model.ErrorResponse
// @Failure      401  {object} model.ErrorResponse
// @Failure      500  {object} model.ErrorResponse
// @Router       /api/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req model.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error: formatValidationError(err),
		})
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	// Buscar usuario por email
	user, err := h.userRepo.FindByEmail(c.Request.Context(), req.Email)
	if err != nil {
		// Retornar el mismo error para no revelar si el email existe o no
		c.JSON(http.StatusUnauthorized, model.ErrorResponse{
			Error: "credenciales inválidas",
		})
		return
	}

	// Comparar la contraseña con el hash almacenado
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, model.ErrorResponse{
			Error: "credenciales inválidas",
		})
		return
	}

	token, err := auth.GenerateToken(user.ID, user.Role)
	if err != nil {
		log.Printf("error al generar JWT: %v", err)
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{
			Error: "error al generar el token de acceso",
		})
		return
	}

	c.JSON(http.StatusOK, model.AuthResponse{
		Token: token,
		User:  *user,
	})
}

// formatValidationError convierte los errores de binding de Gin
// en un mensaje legible para el cliente.
func formatValidationError(err error) string {
	return err.Error()
}
