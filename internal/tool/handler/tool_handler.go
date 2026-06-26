package toolhandler

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/tool-inventory-api/internal/shared"
	sharedmiddleware "github.com/yourusername/tool-inventory-api/internal/shared/middleware"
	toolports "github.com/yourusername/tool-inventory-api/internal/tool/ports"
)

const (
	maxPhotoSize  = 10 << 20 // 10 MB
)

var allowedImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
	"image/gif":  true,
}

type ToolHandler struct {
	toolSvc toolports.ToolService
}

func NewToolHandler(toolSvc toolports.ToolService) *ToolHandler {
	return &ToolHandler{toolSvc: toolSvc}
}

// GetTools godoc
// @Summary      Catálogo de herramientas
// @Description  Lista todas las herramientas. Acepta filtros por disponibilidad, categoría y búsqueda por nombre
// @Tags         herramientas
// @Produce      json
// @Param        available query bool   false "Solo herramientas disponibles"
// @Param        category  query string false "Filtrar por categoría"
// @Param        search    query string false "Buscar por nombre o descripción"
// @Success      200 {array}  ToolResponse
// @Failure      500 {object} dto.ErrResponse
// @Router       /tools [get]
func (h *ToolHandler) GetTools(c *gin.Context) {
	filter := toolports.ToolFilter{
		OnlyAvailable: c.Query("available") == "true",
		Category:      c.Query("category"),
		Search:        c.Query("search"),
	}

	tools, err := h.toolSvc.List(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error al obtener el catálogo"})
		return
	}

	c.JSON(http.StatusOK, ToToolListResponse(tools))
}

// GetMyTools godoc
// @Summary      Mis herramientas
// @Description  Lista las herramientas del propietario autenticado
// @Tags         herramientas
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array}  ToolResponse
// @Failure      401 {object} dto.ErrResponse
// @Failure      500 {object} dto.ErrResponse
// @Router       /owner/tools [get]
func (h *ToolHandler) GetMyTools(c *gin.Context) {
	ownerID := sharedmiddleware.UserIDFromContext(c)
	filter := toolports.ToolFilter{OwnerID: &ownerID}

	tools, err := h.toolSvc.List(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error al obtener tus herramientas"})
		return
	}

	c.JSON(http.StatusOK, ToToolListResponse(tools))
}

// GetTool godoc
// @Summary      Detalle de herramienta
// @Description  Devuelve los datos de una herramienta incluyendo precio sugerido
// @Tags         herramientas
// @Produce      json
// @Param        id path string true "UUID de la herramienta"
// @Success      200 {object} ToolResponse
// @Failure      400 {object} dto.ErrResponse
// @Failure      404 {object} dto.ErrResponse
// @Router       /tools/{id} [get]
func (h *ToolHandler) GetTool(c *gin.Context) {
	id, err := shared.ParseUUID(c, "id")
	if err != nil {
		return
	}

	tool, err := h.toolSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		shared.HandleServiceErr(c, err)
		return
	}

	c.JSON(http.StatusOK, ToToolResponse(tool))
}

// CreateTool godoc
// @Summary      Registrar herramienta
// @Description  Crea una nueva herramienta. El precio diario mínimo se calcula automáticamente (50% del valor estimado en 30 días)
// @Tags         herramientas
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body CreateToolRequest true "Datos de la herramienta"
// @Success      201 {object} ToolResponse
// @Failure      400 {object} dto.ErrResponse
// @Failure      401 {object} dto.ErrResponse
// @Failure      403 {object} dto.ErrResponse
// @Failure      500 {object} dto.ErrResponse
// @Router       /tools [post]
func (h *ToolHandler) CreateTool(c *gin.Context) {
	var req CreateToolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ownerID := sharedmiddleware.UserIDFromContext(c)

	tool, err := h.toolSvc.Create(c.Request.Context(), toolports.CreateToolInput{
		OwnerID:        ownerID,
		Name:           req.Name,
		Description:    req.Description,
		Category:       req.Category,
		EstimatedValue: req.EstimatedValue,
		DailyRate:      req.DailyRate,
		Latitude:       req.Latitude,
		Longitude:      req.Longitude,
	})
	if err != nil {
		log.Printf("ERROR en CreateTool: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, ToToolResponse(tool))
}

// UpdateTool godoc
// @Summary      Actualizar herramienta
// @Description  Modifica los campos de una herramienta. Solo el propietario dueño puede hacerlo
// @Tags         herramientas
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path string              true "UUID de la herramienta"
// @Param        body body UpdateToolRequest true "Campos a actualizar"
// @Success      200 {object} ToolResponse
// @Failure      400 {object} dto.ErrResponse
// @Failure      401 {object} dto.ErrResponse
// @Failure      403 {object} dto.ErrResponse
// @Failure      404 {object} dto.ErrResponse
// @Router       /tools/{id} [put]
// func (h *ToolHandler) UpdateTool(c *gin.Context)
func (h *ToolHandler) UpdateTool(c *gin.Context) {
	id, err := shared.ParseUUID(c, "id")
	if err != nil {
		return
	}

	var req UpdateToolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ownerID := sharedmiddleware.UserIDFromContext(c)

	tool, err := h.toolSvc.Update(c.Request.Context(), id, ownerID, toolports.UpdateToolInput{
		Name:           req.Name,
		Description:    req.Description,
		Category:       req.Category,
		EstimatedValue: req.EstimatedValue,
		DailyRate:      req.DailyRate,
		Latitude:       req.Latitude,
		Longitude:      req.Longitude,
		IsAvailable:    req.IsAvailable,
	})
	if err != nil {
		shared.HandleServiceErr(c, err)
		return
	}

	c.JSON(http.StatusOK, ToToolResponse(tool))
}

// DeleteTool godoc
// @Summary      Eliminar herramienta
// @Description  Elimina una herramienta. Solo el propietario dueño puede hacerlo
// @Tags         herramientas
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "UUID de la herramienta"
// @Success      200 {object} dto.MsgResponse
// @Failure      401 {object} dto.ErrResponse
// @Failure      403 {object} dto.ErrResponse
// @Failure      404 {object} dto.ErrResponse
// @Router       /tools/{id} [delete]
func (h *ToolHandler) DeleteTool(c *gin.Context) {
	id, err := shared.ParseUUID(c, "id")
	if err != nil {
		return
	}

	ownerID := sharedmiddleware.UserIDFromContext(c)

	if err := h.toolSvc.Delete(c.Request.Context(), id, ownerID); err != nil {
		shared.HandleServiceErr(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "herramienta eliminada correctamente"})
}

// UploadPhoto godoc
// @Summary      Subir foto de herramienta
// @Description  Sube una imagen (JPEG, PNG, WEBP o GIF, máx 10 MB). Enviar como multipart/form-data con el campo "photo"
// @Tags         herramientas
// @Accept       multipart/form-data
// @Produce      json
// @Security     BearerAuth
// @Param        id    path     string true  "UUID de la herramienta"
// @Param        photo formData file   true  "Archivo de imagen (JPEG/PNG/WEBP/GIF)"
// @Success      200 {object} ToolResponse
// @Failure      400 {object} dto.ErrResponse "Archivo inválido o tipo no permitido"
// @Failure      401 {object} dto.ErrResponse
// @Failure      403 {object} dto.ErrResponse
// @Failure      404 {object} dto.ErrResponse
// @Failure      413 {object} dto.ErrResponse "Imagen demasiado grande (máx 10 MB)"
// @Router       /tools/{id}/photo [post]
func (h *ToolHandler) UploadPhoto(c *gin.Context) {
	id, err := shared.ParseUUID(c, "id")
	if err != nil {
		return
	}

	// Limitar tamaño del body antes de leer el archivo
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxPhotoSize+1024)

	file, header, err := c.Request.FormFile("photo")
	if err != nil {
		if strings.Contains(err.Error(), "too large") {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "la imagen no puede superar 10 MB"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "se requiere el archivo 'photo'"})
		return
	}
	defer file.Close()

	// Verificar tamaño
	if header.Size > maxPhotoSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "la imagen no puede superar 10 MB"})
		return
	}

	// Leer los primeros 512 bytes para detectar el tipo REAL del archivo
	// (no confiar solo en el Content-Type del cliente)
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no se pudo leer el archivo"})
		return
	}
	detectedType := http.DetectContentType(buf[:n])

	if !allowedImageTypes[detectedType] {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":          "tipo de archivo no permitido",
			"detected_type":  detectedType,
			"allowed_types":  []string{"image/jpeg", "image/png", "image/webp", "image/gif"},
		})
		return
	}

	// Reconstruir el reader para que el servicio reciba el archivo completo
	fullContent := io.MultiReader(bytes.NewReader(buf[:n]), file)

	ownerID := sharedmiddleware.UserIDFromContext(c)

	tool, err := h.toolSvc.UploadPhoto(c.Request.Context(), toolports.UploadPhotoInput{
		ToolID:      id,
		OwnerID:     ownerID,
		Filename:    header.Filename,
		Content:     fullContent,
		ContentType: detectedType,
	})
	if err != nil {
		shared.HandleServiceErr(c, err)
		return
	}

	c.JSON(http.StatusOK, ToToolResponse(tool))
}

// GetPricingSuggestion godoc
// @Summary      Precio sugerido
// @Description  Calcula el precio diario mínimo sugerido basado en el valor estimado de la herramienta (50% del valor en 30 días)
// @Tags         herramientas
// @Produce      json
// @Param        estimated_value query number true "Valor estimado de la herramienta" example(1000)
// @Success      200 {object} input.PricingSuggestion
// @Failure      400 {object} dto.ErrResponse
// @Router       /pricing [get]
func (h *ToolHandler) GetPricingSuggestion(c *gin.Context) {
	var q PricingQueryRequest
	if err := c.ShouldBindQuery(&q); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	suggestion := h.toolSvc.GetPricingSuggestion(c.Request.Context(), q.EstimatedValue, q.ScoreCondicion, q.Category, q.Brand)
	c.JSON(http.StatusOK, suggestion)
}

func (h *ToolHandler) PredictCondition(c *gin.Context) {
	// Limitar tamaño del body a 10MB
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxPhotoSize+1024)

	file, header, err := c.Request.FormFile("photo")
	if err != nil {
		if strings.Contains(err.Error(), "too large") {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "la imagen no puede superar 10 MB"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "se requiere el archivo 'photo'"})
		return
	}
	defer file.Close()

	if header.Size > maxPhotoSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "la imagen no puede superar 10 MB"})
		return
	}

	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no se pudo leer el archivo"})
		return
	}
	detectedType := http.DetectContentType(buf[:n])

	if !allowedImageTypes[detectedType] {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":         "tipo de archivo no permitido",
			"detected_type": detectedType,
		})
		return
	}

	fullContent := io.MultiReader(bytes.NewReader(buf[:n]), file)
	out, err := h.toolSvc.PredictCondition(c.Request.Context(), header.Filename, fullContent, detectedType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, PredictConditionResponse{
		ClasePredicha:  out.ClasePredicha,
		ScoreCondicion: out.ScoreCondicion,
	})
}

func (h *ToolHandler) AutoValuate(c *gin.Context) {
	name := c.Query("name")
	brand := c.Query("brand")
	category := c.Query("category")
	scoreCondicionStr := c.Query("score_condicion")

	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "el parámetro 'name' es requerido"})
		return
	}

	var scoreCondicion float64 = 0.70
	if scoreCondicionStr != "" {
		var err error
		scoreCondicion, err = strconv.ParseFloat(scoreCondicionStr, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "el parámetro 'score_condicion' debe ser numérico"})
			return
		}
	}

	out, err := h.toolSvc.AutoValuate(c.Request.Context(), name, scoreCondicion, category, brand)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, AutoValuateResponse{
		EstimatedValue: out.EstimatedValue,
		SuggestedDaily: out.SuggestedDaily,
		MinimumDaily:   out.MinimumDaily,
		Description:    out.Description,
	})
}
