package handler

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yourusername/tool-inventory-api/internal/middleware"
	"github.com/yourusername/tool-inventory-api/internal/model"
	"github.com/yourusername/tool-inventory-api/internal/repository"
)

// ToolHandler agrupa los handlers relacionados con herramientas.
type ToolHandler struct {
	toolRepo *repository.ToolRepository
}

// NewToolHandler crea una nueva instancia del handler de herramientas.
func NewToolHandler(toolRepo *repository.ToolRepository) *ToolHandler {
	return &ToolHandler{toolRepo: toolRepo}
}

// GetTools godoc
// @Summary      Listar herramientas
// @Description  Retorna el catálogo completo de herramientas. Accesible por todos.
// @Tags         tools
// @Produce      json
// @Param        available query bool false "Filtrar solo disponibles"
// @Success      200 {array}  model.Tool
// @Failure      500 {object} model.ErrorResponse
// @Router       /api/tools [get]
func (h *ToolHandler) GetTools(c *gin.Context) {
	onlyAvailable := c.Query("available") == "true"

	tools, err := h.toolRepo.FindAll(c.Request.Context(), onlyAvailable)
	if err != nil {
		log.Printf("error al obtener herramientas: %v", err)
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{
			Error: "error al obtener el catálogo de herramientas",
		})
		return
	}

	c.JSON(http.StatusOK, tools)
}

// GetMyTools godoc
// @Summary      Mis herramientas
// @Description  Retorna únicamente las herramientas que pertenecen al propietario autenticado (owner_id == user_id del token).
// @Tags         tools
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array}  model.Tool
// @Failure      401 {object} model.ErrorResponse
// @Failure      403 {object} model.ErrorResponse
// @Failure      500 {object} model.ErrorResponse
// @Router       /api/tools/mine [get]
func (h *ToolHandler) GetMyTools(c *gin.Context) {
	// El ownerID viene directamente del JWT — nunca del query param ni del body
	ownerID := c.MustGet(middleware.ContextUserID).(uuid.UUID)

	tools, err := h.toolRepo.FindByOwnerID(c.Request.Context(), ownerID)
	if err != nil {
		log.Printf("error al obtener herramientas del propietario %s: %v", ownerID, err)
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{
			Error: "error al obtener tus herramientas",
		})
		return
	}

	c.JSON(http.StatusOK, tools)
}

// SuggestPrice godoc
// @Summary      Precio sugerido por IA
// @Description  Devuelve un precio sugerido basado en categoría, nivel de desgaste y marca
// @Tags         tools
// @Produce      json
// @Security     BearerAuth
// @Param        category   query string false "Categoría de la herramienta"
// @Param        wear_level query string false "Nivel de desgaste"
// @Param        brand      query string false "Marca"
// @Success      200 {object} model.SuggestPriceResponse
// @Router       /api/tools/suggest-price [get]
func (h *ToolHandler) SuggestPrice(c *gin.Context) {
	category := c.Query("category")
	wearLevel := c.Query("wear_level")

	// Tabla base de precios por categoría (MXN por día)
	basePrices := map[string]float64{
		"Eléctrico":   350.0,
		"Manual":      120.0,
		"Jardinería":  180.0,
		"Construcción": 450.0,
		"Medición":    200.0,
		"Otro":        150.0,
	}

	base, found := basePrices[category]
	if !found {
		base = 200.0 // precio genérico si categoría no reconocida
	}

	// Multiplicador por nivel de desgaste
	multiplier := 1.0
	switch wearLevel {
	case "Nuevo":
		multiplier = 1.2
	case "Buen Estado":
		multiplier = 1.0
	case "Desgastado":
		multiplier = 0.7
	}

	suggested := math.Round(base*multiplier*100) / 100

	c.JSON(http.StatusOK, model.SuggestPriceResponse{
		SuggestedPrice: suggested,
		Currency:       "MXN",
		Unit:           "por día",
	})
}

// CreateTool godoc
// @Summary      Crear herramienta
// @Description  Crea una nueva herramienta. Solo para Propietarios.
// @Tags         tools
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body model.CreateToolRequest true "Datos de la herramienta"
// @Success      201 {object} model.Tool
// @Failure      400 {object} model.ErrorResponse
// @Failure      401 {object} model.ErrorResponse
// @Failure      403 {object} model.ErrorResponse
// @Failure      500 {object} model.ErrorResponse
// @Router       /api/tools [post]
func (h *ToolHandler) CreateTool(c *gin.Context) {
	var req model.CreateToolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error: formatValidationError(err),
		})
		return
	}

	ownerID := c.MustGet(middleware.ContextUserID).(uuid.UUID)

	isAvailable := true
	if req.IsAvailable != nil {
		isAvailable = *req.IsAvailable
	}

	tool := &model.Tool{
		OwnerID:     ownerID,
		Name:        req.Name,
		Description: req.Description,
		Category:    req.Category,
		IsAvailable: isAvailable,
		Brand:       req.Brand,
		Model:       req.Model,
		WearLevel:   req.WearLevel,
		Latitude:    req.Latitude,
		Longitude:   req.Longitude,
		PricePerDay: req.PricePerDay,
	}

	created, err := h.toolRepo.Create(c.Request.Context(), tool)
	if err != nil {
		log.Printf("error al crear herramienta: %v", err)
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{
			Error: "error al crear la herramienta",
		})
		return
	}

	c.JSON(http.StatusCreated, created)
}

// UpdateTool godoc
// @Summary      Actualizar herramienta
// @Description  Actualiza los datos de una herramienta. Solo el Propietario dueño puede modificarla.
// @Tags         tools
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path string                  true "UUID de la herramienta"
// @Param        body body model.UpdateToolRequest true "Campos a actualizar"
// @Success      200 {object} model.Tool
// @Failure      400 {object} model.ErrorResponse
// @Failure      403 {object} model.ErrorResponse
// @Failure      404 {object} model.ErrorResponse
// @Failure      500 {object} model.ErrorResponse
// @Router       /api/tools/{id} [put]
func (h *ToolHandler) UpdateTool(c *gin.Context) {
	toolID, err := parseUUID(c, "id")
	if err != nil {
		return
	}

	var req model.UpdateToolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error: formatValidationError(err),
		})
		return
	}

	ownerID := c.MustGet(middleware.ContextUserID).(uuid.UUID)

	updated, err := h.toolRepo.Update(c.Request.Context(), toolID, ownerID, &req)
	if err != nil {
		handleRepoError(c, err)
		return
	}

	c.JSON(http.StatusOK, updated)
}

// UploadPhoto godoc
// @Summary      Subir foto de herramienta
// @Description  Sube una imagen para la herramienta especificada. Multipart/form-data.
// @Tags         tools
// @Accept       multipart/form-data
// @Produce      json
// @Security     BearerAuth
// @Param        id    path     string true  "UUID de la herramienta"
// @Param        photo formData file   true  "Archivo de imagen"
// @Success      200 {object} model.PhotoUploadResponse
// @Failure      400 {object} model.ErrorResponse
// @Failure      403 {object} model.ErrorResponse
// @Failure      500 {object} model.ErrorResponse
// @Router       /api/tools/{id}/photo [post]
func (h *ToolHandler) UploadPhoto(c *gin.Context) {
	toolID, err := parseUUID(c, "id")
	if err != nil {
		return
	}

	ownerID := c.MustGet(middleware.ContextUserID).(uuid.UUID)

	// Leer el archivo del form
	file, header, err := c.Request.FormFile("photo")
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error: "se requiere el campo 'photo' con un archivo de imagen",
		})
		return
	}
	defer file.Close()

	// Validar extensión
	ext := filepath.Ext(header.Filename)
	allowedExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}
	if !allowedExts[ext] {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error: "formato de imagen no permitido (use jpg, jpeg, png o webp)",
		})
		return
	}

	// Crear directorio de uploads si no existe
	uploadDir := "./uploads/tools"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Printf("error al crear directorio de uploads: %v", err)
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{
			Error: "error interno al procesar la imagen",
		})
		return
	}

	// Guardar el archivo con nombre único (UUID + timestamp)
	filename := fmt.Sprintf("%s_%d%s", toolID.String(), time.Now().Unix(), ext)
	destPath := filepath.Join(uploadDir, filename)

	dest, err := os.Create(destPath)
	if err != nil {
		log.Printf("error al crear archivo destino: %v", err)
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{
			Error: "error interno al guardar la imagen",
		})
		return
	}
	defer dest.Close()

	if _, err := io.Copy(dest, file); err != nil {
		log.Printf("error al copiar archivo: %v", err)
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{
			Error: "error interno al guardar la imagen",
		})
		return
	}

	// Construir URL pública
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	photoURL := fmt.Sprintf("%s/uploads/tools/%s", baseURL, filename)

	// Actualizar la herramienta en BD
	if _, err := h.toolRepo.UpdatePhoto(c.Request.Context(), toolID, ownerID, photoURL); err != nil {
		handleRepoError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.PhotoUploadResponse{
		PhotoURL: photoURL,
	})
}

// DeleteTool godoc
// @Summary      Eliminar herramienta
// @Description  Elimina una herramienta. Solo el Propietario dueño puede eliminarla.
// @Tags         tools
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "UUID de la herramienta"
// @Success      200 {object} model.SuccessResponse
// @Failure      403 {object} model.ErrorResponse
// @Failure      404 {object} model.ErrorResponse
// @Failure      500 {object} model.ErrorResponse
// @Router       /api/tools/{id} [delete]
func (h *ToolHandler) DeleteTool(c *gin.Context) {
	toolID, err := parseUUID(c, "id")
	if err != nil {
		return
	}

	ownerID := c.MustGet(middleware.ContextUserID).(uuid.UUID)

	if err := h.toolRepo.Delete(c.Request.Context(), toolID, ownerID); err != nil {
		handleRepoError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.SuccessResponse{
		Message: "herramienta eliminada correctamente",
	})
}

// =============================================================================
// Helpers privados
// =============================================================================

// parseUUID extrae y valida un parámetro UUID de la ruta.
func parseUUID(c *gin.Context, param string) (uuid.UUID, error) {
	raw := c.Param(param)
	id, err := uuid.Parse(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error: "el ID proporcionado no es un UUID válido",
		})
		return uuid.Nil, err
	}
	return id, nil
}

// handleRepoError mapea los errores del repositorio a respuestas HTTP semánticas.
func handleRepoError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		c.JSON(http.StatusNotFound, model.ErrorResponse{
			Error: "herramienta no encontrada",
		})
	case errors.Is(err, repository.ErrForbidden):
		c.JSON(http.StatusForbidden, model.ErrorResponse{
			Error: "no tienes permiso sobre esta herramienta",
		})
	default:
		log.Printf("error de repositorio: %v", err)
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{
			Error: "error interno del servidor",
		})
	}
}

// generateContractHash genera un hash SHA-256 para el contrato digital de entrega.
func generateContractHash(rentalID uuid.UUID, lat, lng float64) string {
	data := fmt.Sprintf("%s|%.7f|%.7f|%d", rentalID.String(), lat, lng, time.Now().Unix())
	h := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%X", h)
}

// parsePriceQuery lee el query param "price" como float64.
func parsePriceQuery(c *gin.Context, param string) (float64, error) {
	raw := c.Query(param)
	if raw == "" {
		return 0, nil
	}
	val, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("el parámetro '%s' debe ser un número válido", param)
	}
	return val, nil
}
