package toolhandler

import (
	"bytes"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/tool-inventory-api/internal/shared"
	apperrors "github.com/yourusername/tool-inventory-api/internal/shared/errors"
	sharedmiddleware "github.com/yourusername/tool-inventory-api/internal/shared/middleware"
	toolports "github.com/yourusername/tool-inventory-api/internal/tool/ports"
	toolservice "github.com/yourusername/tool-inventory-api/internal/tool/service"
)

const (
	maxPhotoSize = 10 << 20 // 10 MB
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

	// "Mis herramientas" está acotado al propio propietario, así que el N+1
	// aquí es aceptable — la pantalla de Editar necesita ver todas las fotos
	// reales (no solo la portada) para no pedirlas de nuevo.
	resp := ToToolListResponse(tools)
	for i, t := range tools {
		if photos, err := h.toolSvc.GetPhotos(c.Request.Context(), t.ID); err == nil {
			resp[i].Photos = ToToolPhotoResponse(photos)
		}
	}

	c.JSON(http.StatusOK, resp)
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

	resp := ToToolResponse(tool)
	if photos, err := h.toolSvc.GetPhotos(c.Request.Context(), id); err == nil {
		resp.Photos = ToToolPhotoResponse(photos)
	}

	c.JSON(http.StatusOK, resp)
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
		Brand:          req.Brand,
		AgeMonths:      req.AgeMonths,
		City:           req.City,
		State:          req.State,
		ConditionScore: req.ConditionScore,
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
			"error":         "tipo de archivo no permitido",
			"detected_type": detectedType,
			"allowed_types": []string{"image/jpeg", "image/png", "image/webp", "image/gif"},
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

	resp := ToToolResponse(tool)
	if photos, err := h.toolSvc.GetPhotos(c.Request.Context(), id); err == nil {
		resp.Photos = ToToolPhotoResponse(photos)
	}

	c.JSON(http.StatusOK, resp)
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

	name := q.Name
	if name == "" {
		name = q.Category
	}
	ageMonths := q.AgeMonths
	if ageMonths <= 0 {
		ageMonths = 12
	}

	suggestion := h.toolSvc.GetPricingSuggestion(c.Request.Context(), q.EstimatedValue, q.ScoreCondicion, q.Category, q.Brand, name, ageMonths)
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
	ageMonthsStr := c.Query("age_months")
	precioBaseManualStr := c.Query("precio_base_manual")
	ticketValidadoStr := c.Query("ticket_validado")

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

	var ageMonths int = 12
	if ageMonthsStr != "" {
		var err error
		ageMonths, err = strconv.Atoi(ageMonthsStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "el parámetro 'age_months' debe ser un número entero"})
			return
		}
	}

	// precio_base_manual llega junto con ticket_validado=true cuando el
	// propietario confirmó el monto detectado por /tools/extract-ticket-price.
	var precioBaseManual *float64
	if precioBaseManualStr != "" {
		val, err := strconv.ParseFloat(precioBaseManualStr, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "el parámetro 'precio_base_manual' debe ser numérico"})
			return
		}
		precioBaseManual = &val
	}
	ticketValidado := ticketValidadoStr == "true"

	out, err := h.toolSvc.AutoValuate(c.Request.Context(), name, scoreCondicion, category, brand, ageMonths, precioBaseManual, ticketValidado)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, AutoValuateResponse{
		EstimatedValue:       out.EstimatedValue,
		SuggestedDaily:       out.SuggestedDaily,
		MinimumDaily:         out.MinimumDaily,
		RequiresManualReview: out.RequiresManualReview,
		Description:          out.Description,
	})
}

// ExtractTicketPrice godoc
// @Summary      Iniciar lectura de un ticket de compra
// @Description  Recibe la foto del ticket y arranca el OCR en segundo plano, devolviendo un job_id de inmediato (ver GetTicketPriceJob para el resultado). Asincrono a propósito: el OCR puede tardar decenas de segundos por el arranque en frío del worker de PaddleOCR, y una sola petición HTTP tan larga corre el riesgo de que algún proxy intermedio la corte a medias.
// @Tags         herramientas
// @Accept       multipart/form-data
// @Produce      json
// @Security     BearerAuth
// @Param        photo formData file true "Foto del ticket de compra (JPEG/PNG/WEBP/GIF)"
// @Success      202 {object} TicketJobStartedResponse
// @Failure      400 {object} dto.ErrResponse "Archivo inválido o tipo no permitido"
// @Failure      401 {object} dto.ErrResponse
// @Failure      413 {object} dto.ErrResponse "Imagen demasiado grande (máx 10 MB)"
// @Router       /tools/extract-ticket-price [post]
func (h *ToolHandler) ExtractTicketPrice(c *gin.Context) {
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

	// A diferencia del resto de los endpoints de ML, aquí se lee TODO el
	// archivo a memoria antes de responder: el io.Reader del multipart deja
	// de ser válido en cuanto este handler termina, pero el OCR real sigue
	// corriendo en segundo plano después de eso.
	restoDelArchivo, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no se pudo leer el archivo"})
		return
	}
	contenidoCompleto := append(buf[:n], restoDelArchivo...)

	jobID := h.toolSvc.StartTicketPriceJob(header.Filename, contenidoCompleto, detectedType)
	c.JSON(http.StatusAccepted, TicketJobStartedResponse{JobID: jobID})
}

// GetTicketPriceJob godoc
// @Summary      Consultar el resultado de un OCR de ticket iniciado
// @Description  El cliente pregunta cada pocos segundos hasta que status ya no sea "processing"
// @Tags         herramientas
// @Produce      json
// @Security     BearerAuth
// @Param        job_id path string true "job_id devuelto por POST /tools/extract-ticket-price"
// @Success      200 {object} TicketJobStatusResponse
// @Failure      404 {object} dto.ErrResponse "job_id no encontrado (expiró o nunca existió)"
// @Router       /tools/extract-ticket-price/{job_id} [get]
func (h *ToolHandler) GetTicketPriceJob(c *gin.Context) {
	jobID := c.Param("job_id")

	status, result, errMsg, found := h.toolSvc.GetTicketPriceJob(jobID)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "job no encontrado o expirado"})
		return
	}

	resp := TicketJobStatusResponse{Status: status, Error: errMsg}
	if result != nil {
		resp.Valid = result.Valid
		resp.DetectedPrice = result.DetectedPrice
		resp.Confidence = result.Confidence
		if result.Error != "" {
			resp.Error = result.Error
		}
	}
	c.JSON(http.StatusOK, resp)
}

// CreateInsurancePreference godoc
// @Summary      Crear preferencia de pago para el seguro
// @Description  Crea una preferencia en Mercado Pago para pagar la prima mensual del seguro de esta herramienta y devuelve el init_point para el WebView
// @Tags         herramientas
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "UUID de la herramienta"
// @Success      200 {object} InsurancePreferenceResponse
// @Failure      401 {object} dto.ErrResponse
// @Failure      403 {object} dto.ErrResponse
// @Failure      422 {object} dto.ErrResponse "Error al crear preferencia en MP"
// @Router       /tools/{id}/insurance/preference [post]
func (h *ToolHandler) CreateInsurancePreference(c *gin.Context) {
	id, err := shared.ParseUUID(c, "id")
	if err != nil {
		return
	}
	ownerID := sharedmiddleware.UserIDFromContext(c)

	out, err := h.toolSvc.CreateInsurancePreference(c.Request.Context(), id, ownerID)
	if err != nil {
		switch {
		case errors.Is(err, apperrors.ErrForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case errors.Is(err, toolservice.ErrInsurancePayment):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, InsurancePreferenceResponse{
		InitPoint:    out.InitPoint,
		PreferenceID: out.PreferenceID,
	})
}

// ConfirmInsurancePayment godoc
// @Summary      Confirmar pago del seguro
// @Description  Verifica el estado de un pago directamente contra Mercado Pago y activa el seguro de la herramienta si está aprobado
// @Tags         herramientas
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "UUID de la herramienta"
// @Param        body body ConfirmInsuranceRequest true "ID del pago devuelto por MP"
// @Success      200 {object} ToolResponse
// @Failure      401 {object} dto.ErrResponse
// @Failure      402 {object} dto.ErrResponse "Pago no aprobado o no corresponde a esta herramienta"
// @Router       /tools/{id}/insurance/confirm [post]
func (h *ToolHandler) ConfirmInsurancePayment(c *gin.Context) {
	id, err := shared.ParseUUID(c, "id")
	if err != nil {
		return
	}
	ownerID := sharedmiddleware.UserIDFromContext(c)

	var req ConfirmInsuranceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tool, err := h.toolSvc.ConfirmInsurancePayment(c.Request.Context(), id, ownerID, req.PaymentID)
	if err != nil {
		switch {
		case errors.Is(err, apperrors.ErrForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case errors.Is(err, toolservice.ErrInsuranceNotApproved), errors.Is(err, toolservice.ErrInsuranceRefMismatch):
			c.JSON(http.StatusPaymentRequired, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, ToToolResponse(tool))
}

// ReconcileInsurance godoc
// @Summary      Reconciliar el pago del seguro sin payment_id
// @Description  Busca el pago del seguro en Mercado Pago por external_reference y activa el seguro si está aprobado. Lo usa la app al regresar del checkout cuando no se pudo interceptar el payment_id.
// @Tags         herramientas
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "UUID de la herramienta"
// @Success      200 {object} ToolResponse
// @Router       /tools/{id}/insurance/reconcile [post]
func (h *ToolHandler) ReconcileInsurance(c *gin.Context) {
	id, err := shared.ParseUUID(c, "id")
	if err != nil {
		return
	}
	ownerID := sharedmiddleware.UserIDFromContext(c)

	tool, err := h.toolSvc.ReconcileInsurance(c.Request.Context(), id, ownerID)
	if err != nil {
		switch {
		case errors.Is(err, apperrors.ErrForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, ToToolResponse(tool))
}

// CancelInsurance godoc
// @Summary      Cancelar el seguro de una herramienta
// @Description  Desactiva el seguro; no genera reembolso de la prima ya pagada, solo detiene la cobertura
// @Tags         herramientas
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "UUID de la herramienta"
// @Success      200 {object} ToolResponse
// @Failure      401 {object} dto.ErrResponse
// @Failure      403 {object} dto.ErrResponse
// @Router       /tools/{id}/insurance/cancel [post]
func (h *ToolHandler) CancelInsurance(c *gin.Context) {
	id, err := shared.ParseUUID(c, "id")
	if err != nil {
		return
	}
	ownerID := sharedmiddleware.UserIDFromContext(c)

	tool, err := h.toolSvc.CancelInsurance(c.Request.Context(), id, ownerID)
	if err != nil {
		switch {
		case errors.Is(err, apperrors.ErrForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, ToToolResponse(tool))
}
