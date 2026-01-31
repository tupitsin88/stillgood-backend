package categories

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// @Summary Get all categories
// @Tags Categories
// @Produce json
// @Success 200 {object} map[string][]CategoryResponse
// @Router /categories [get]
func (h *Handler) GetList(c *gin.Context) {
	categories, err := h.service.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch categories"})
		return
	}

	var response []CategoryResponse
	for _, cat := range categories {
		response = append(response, CategoryResponse{
			ID:      cat.ID,
			Name:    cat.Name,
			IconURL: cat.IconURL,
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": response})
}
