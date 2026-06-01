package handler

import (
	"errors"
	"log"
	"net/http"

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
	// Parámetro de query opcional: ?available=true
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

	// Valor por defecto para is_available: true si no se envía en el body
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
		return // parseUUID ya escribió la respuesta de error
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
// Si es inválido, escribe la respuesta 400 y retorna error.
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
