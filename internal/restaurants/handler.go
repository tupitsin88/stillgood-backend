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
	router := r.Group("/api/v1/restaurants")
	{
		router.GET("", h.GetList)
		router.GET("/:id", h.GetByID)
		router.POST("/upload", h.UploadImage)
	}
}

// @Summary Upload image
// @Tags restaurants
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
		ID:        restaurant.ID.String(),
		Name:      restaurant.Name,
		Address:   restaurant.Address,
		Phone:     restaurant.Phone,
		Latitude:  restaurant.Latitude,
		Longitude: restaurant.Longitude,
	}

	c.JSON(http.StatusOK, response)
}
