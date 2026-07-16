package reviewhandler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/tool-inventory-api/internal/shared"
	sharedmiddleware "github.com/yourusername/tool-inventory-api/internal/shared/middleware"
	reviewports "github.com/yourusername/tool-inventory-api/internal/review/ports"
	reviewservice "github.com/yourusername/tool-inventory-api/internal/review/service"
)

type ReviewHandler struct {
	reviewSvc reviewports.ReviewService
}

func NewReviewHandler(reviewSvc reviewports.ReviewService) *ReviewHandler {
	return &ReviewHandler{reviewSvc: reviewSvc}
}

// CreateReview godoc
// @Summary      Calificar una renta finalizada
// @Description  Si el autor es el propietario, califica al solicitante (reputación). Si el autor es el solicitante, califica la herramienta. Solo permitido cuando la renta está en estado "completed", y una vez por dirección
// @Tags         reseñas
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path string              true "UUID de la renta"
// @Param        body body CreateReviewRequest true "Calificación (1-5 estrellas) y comentario opcional"
// @Success      201 {object} ReviewResponse
// @Failure      400 {object} dto.ErrResponse
// @Failure      401 {object} dto.ErrResponse
// @Failure      403 {object} dto.ErrResponse
// @Failure      404 {object} dto.ErrResponse
// @Failure      409 {object} dto.ErrResponse
// @Router       /rentals/{id}/review [post]
func (h *ReviewHandler) CreateReview(c *gin.Context) {
	rentalID, err := shared.ParseUUID(c, "id")
	if err != nil {
		return
	}

	var req CreateReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	authorID := sharedmiddleware.UserIDFromContext(c)

	review, err := h.reviewSvc.Create(c.Request.Context(), reviewports.CreateReviewInput{
		RentalID: rentalID,
		AuthorID: authorID,
		Rating:   req.Rating,
		Comment:  req.Comment,
	})
	if err != nil {
		switch {
		case errors.Is(err, reviewservice.ErrRentalNotCompleted), errors.Is(err, reviewservice.ErrInvalidRating):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, reviewservice.ErrAlreadyReviewed):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			shared.HandleServiceErr(c, err)
		}
		return
	}

	c.JSON(http.StatusCreated, ToReviewResponse(review))
}

// GetToolReviews godoc
// @Summary      Reseñas de una herramienta
// @Description  Lista las reseñas dejadas por solicitantes al terminar su renta, con el promedio de calificación
// @Tags         reseñas
// @Produce      json
// @Param        id path string true "UUID de la herramienta"
// @Success      200 {object} ReviewListResponse
// @Failure      400 {object} dto.ErrResponse
// @Router       /tools/{id}/reviews [get]
func (h *ReviewHandler) GetToolReviews(c *gin.Context) {
	toolID, err := shared.ParseUUID(c, "id")
	if err != nil {
		return
	}

	reviews, summary, err := h.reviewSvc.ListForTool(c.Request.Context(), toolID)
	if err != nil {
		shared.HandleServiceErr(c, err)
		return
	}

	c.JSON(http.StatusOK, ToReviewListResponse(reviews, summary))
}

// GetUserReviews godoc
// @Summary      Calificaciones de un usuario como solicitante
// @Description  Lista las calificaciones que los propietarios han dejado sobre un solicitante, con el promedio
// @Tags         reseñas
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "UUID del usuario"
// @Success      200 {object} ReviewListResponse
// @Failure      400 {object} dto.ErrResponse
// @Router       /users/{id}/reviews [get]
func (h *ReviewHandler) GetUserReviews(c *gin.Context) {
	userID, err := shared.ParseUUID(c, "id")
	if err != nil {
		return
	}

	reviews, summary, err := h.reviewSvc.ListForUser(c.Request.Context(), userID)
	if err != nil {
		shared.HandleServiceErr(c, err)
		return
	}

	c.JSON(http.StatusOK, ToReviewListResponse(reviews, summary))
}
