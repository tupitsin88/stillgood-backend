package categories

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// GetList @Summary Список категорий
// @Tags Categories
// @Produce json
// @Success 200 {object} map[string]interface{}
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

// Create @Summary Создание категории (Admin Only)
// @Tags Admin
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param input body CreateCategoryRequest true "Данные новой категории"
// @Success 201 {object} domain.Category
// @Router /categories [post]
func (h *Handler) Create(c *gin.Context) {
	var req CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_INPUT"})
		return
	}
	category, err := h.service.Create(req.Name, req.IconURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, category)
}

// Update @Summary Обновление категории (Admin Only)
// @Tags Admin
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path string true "Category ID"
// @Param input body CreateCategoryRequest true "Новые данные"
// @Success 200 {object} domain.Category
// @Router /categories/{id} [patch]
func (h *Handler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_ID"})
		return
	}
	var req CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_INPUT"})
		return
	}
	category, err := h.service.Update(id, req.Name, req.IconURL)
	if err != nil {
		if err.Error() == "record not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "CATEGORY_NOT_FOUND"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, category)
}

// Delete @Summary Удаление категории (Admin Only)
// @Description Нельзя удалить категорию, если в ней есть офферы
// @Tags Admin
// @Security ApiKeyAuth
// @Param id path string true "Category ID"
// @Success 204 "Успешное удаление без тела ответа"
// @Router /categories/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_ID"})
		return
	}
	if err := h.service.Delete(id); err != nil {
		if err.Error() == "CANNOT_DELETE_CATEGORY_WITH_ACTIVE_OFFERS" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "CATEGORY_NOT_EMPTY", "message": "Нельзя удалить категорию с активными боксами"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(204)
}
