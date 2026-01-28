package restaurants

import (
	"net/http"

	"kursach_backend/internal/restaurants/dto"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	// Публичные маршруты
	routes := r.Group("/api/v1/restaurants")
	{
		routes.GET("", h.GetList)
		routes.GET("/:id", h.GetByID)
	}
}

// @Summary Get list of restaurants
// @Tags restaurants
// @Produce json
// @Success 200 {array} dto.RestaurantResponse
// @Router /restaurants [get]
func (h *Handler) GetList(c *gin.Context) {
	restaurants, err := h.service.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Mapping to DTO
	var response []dto.RestaurantResponse
	for _, r := range restaurants {
		response = append(response, dto.RestaurantResponse{
			ID:          r.ID,
			Name:        r.Name,
			Description: r.Description,
			Address:     r.Address,
			City:        r.City,
			Phone:       r.Phone,
			Latitude:    r.Latitude,
			Longitude:   r.Longitude,
			ImageURL:    r.ImageURL,
			Rating:      r.Rating,
		})
	}

	c.JSON(http.StatusOK, response)
}

// @Summary Get restaurant by ID
// @Tags restaurants
// @Produce json
// @Param id path string true "Restaurant ID"
// @Success 200 {object} dto.RestaurantResponse
// @Failure 404 {object} map[string]string
// @Router /restaurants/{id} [get]
func (h *Handler) GetByID(c *gin.Context) {
	id := c.Param("id")

	restaurant, err := h.service.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Restaurant not found"})
		return
	}

	response := dto.RestaurantResponse{
		ID:          restaurant.ID,
		Name:        restaurant.Name,
		Description: restaurant.Description,
		Address:     restaurant.Address,
		City:        restaurant.City,
		Phone:       restaurant.Phone,
		Latitude:    restaurant.Latitude,
		Longitude:   restaurant.Longitude,
		ImageURL:    restaurant.ImageURL,
		Rating:      restaurant.Rating,
	}

	c.JSON(http.StatusOK, response)
}
