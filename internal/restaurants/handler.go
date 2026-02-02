package restaurants

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

// @Summary Загрузка изображения
// @Tags Restaurants
// @Accept multipart/form-data
// @Produce json
// @Param image formData file true "Image file"
// @Success 200 {object} map[string]string
// @Router /restaurants/upload [post]
func (h *Handler) UploadImage(c *gin.Context) {
	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	url, err := h.service.UploadImage(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload image"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": url})
}

// @Summary Список ресторанов
// @Tags Restaurants
// @Produce json
// @Param lat query number false "Latitude"
// @Param lng query number false "Longitude"
// @Param radius query integer false "Радиус в метрах"
// @Param categoryId query string false "Category ID"
// @Param limit query integer false "Limit"
// @Param offset query integer false "Offset"
// @Success 200 {array} RestaurantResponse
// @Router /restaurants [get]
func (h *Handler) GetList(c *gin.Context) {
	restaurants, err := h.service.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Mapping to DTO
	var response []RestaurantResponse
	for _, r := range restaurants {
		response = append(response, RestaurantResponse{
			ID:        r.ID.String(),
			Name:      r.Name,
			Address:   r.Address,
			Phone:     r.Phone,
			Latitude:  r.Latitude,
			Longitude: r.Longitude,
		})
	}

	c.JSON(http.StatusOK, response)
}

// @Summary Детали ресторана
// @Tags Restaurants
// @Produce json
// @Param id path string true "Restaurant ID"
// @Success 200 {object} RestaurantResponse
// @Failure 404 {object} map[string]string
// @Router /restaurants/{id} [get]
func (h *Handler) GetByID(c *gin.Context) {
	id := c.Param("id")

	restaurant, err := h.service.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Restaurant not found"})
		return
	}

	response := RestaurantResponse{
		ID:        restaurant.ID.String(),
		Name:      restaurant.Name,
		Address:   restaurant.Address,
		Phone:     restaurant.Phone,
		Latitude:  restaurant.Latitude,
		Longitude: restaurant.Longitude,
	}

	c.JSON(http.StatusOK, response)
}
