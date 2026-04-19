package restaurants

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) requirePartner(c *gin.Context) (string, bool) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "UNAUTHORIZED"})
		return "", false
	}
	if c.GetString("role") != "PARTNER" {
		c.JSON(http.StatusForbidden, gin.H{"error": "FORBIDDEN", "message": "Partner role required"})
		return "", false
	}

	isApprovedPartner, err := h.service.IsApprovedPartner(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify user role"})
		return "", false
	}
	if !isApprovedPartner {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "PARTNER_NOT_APPROVED",
			"message": "Partner is not approved",
		})
		return "", false
	}

	return userID, true
}

// @Summary Загрузка изображения
// @Tags Restaurants
// @Accept multipart/form-data
// @Produce json
// @Param image formData file true "Image file (JPG, PNG, HEIC/HEIF, max 10MB)"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 413 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /restaurants/upload [post]
func (h *Handler) UploadImage(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadRequestBodyBytes)

	file, err := c.FormFile("image")
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) || strings.Contains(strings.ToLower(err.Error()), "request body too large") {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "Image is too large"})
			return
		}

		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	url, err := h.service.UploadImage(file)
	if err != nil {
		if errors.Is(err, ErrInvalidImageFormat) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid image format"})
			return
		}
		if errors.Is(err, ErrImageTooLarge) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "Image is too large"})
			return
		}
		if errors.Is(err, ErrStorageUnavailable) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Storage is unavailable"})
			return
		}
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
// @Success 200 {object} RestaurantListResponse
// @Router /restaurants [get]
func (h *Handler) GetList(c *gin.Context) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil || limit <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid limit"})
		return
	}
	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil || offset < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid offset"})
		return
	}

	params := ListParams{
		Limit:  limit,
		Offset: offset,
	}

	if latStr := c.Query("lat"); latStr != "" {
		lat, parseErr := strconv.ParseFloat(latStr, 64)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid lat"})
			return
		}
		params.Lat = &lat
	}

	if lngStr := c.Query("lng"); lngStr != "" {
		lng, parseErr := strconv.ParseFloat(lngStr, 64)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid lng"})
			return
		}
		params.Lng = &lng
	}

	if radiusStr := c.Query("radius"); radiusStr != "" {
		radius, parseErr := strconv.Atoi(radiusStr)
		if parseErr != nil || radius <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid radius"})
			return
		}
		params.Radius = &radius
	}

	if categoryIDStr := c.Query("categoryId"); categoryIDStr != "" {
		categoryID, parseErr := uuid.Parse(categoryIDStr)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid categoryId"})
			return
		}
		params.CategoryID = &categoryID
	}

	restaurants, total, err := h.service.GetList(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	restaurantIDs := make([]uuid.UUID, 0, len(restaurants))
	for _, r := range restaurants {
		restaurantIDs = append(restaurantIDs, r.ID)
	}
	metaByRestaurantID, err := h.service.GetOfferMetaByRestaurantIDs(restaurantIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to aggregate restaurant metadata"})
		return
	}

	var response []RestaurantResponse
	for _, r := range restaurants {
		var distance *int
		if params.Lat != nil && params.Lng != nil {
			val := int(calculateDistance(*params.Lat, *params.Lng, r.Latitude, r.Longitude))
			distance = &val
		}
		meta := metaByRestaurantID[r.ID]
		response = append(response, RestaurantResponse{
			ID:              r.ID.String(),
			Name:            r.Name,
			Address:         r.Address,
			Phone:           r.Phone,
			ImageURL:        r.ImageURL,
			Description:     r.Description,
			WorkingHours:    r.WorkingHours,
			Latitude:        r.Latitude,
			Longitude:       r.Longitude,
			Rating:          r.Rating,
			ReviewCount:     r.ReviewCount,
			Categories:      meta.Categories,
			HasActiveOffers: meta.HasActiveOffers,
			Distance:        distance,
		})
	}

	c.JSON(http.StatusOK, RestaurantListResponse{
		Data: response,
		Pagination: Pagination{
			Total:  total,
			Limit:  limit,
			Offset: offset,
		},
	})
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

	metaByRestaurantID, err := h.service.GetOfferMetaByRestaurantIDs([]uuid.UUID{restaurant.ID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to aggregate restaurant metadata"})
		return
	}
	meta := metaByRestaurantID[restaurant.ID]

	response := RestaurantResponse{
		ID:              restaurant.ID.String(),
		Name:            restaurant.Name,
		Address:         restaurant.Address,
		Phone:           restaurant.Phone,
		ImageURL:        restaurant.ImageURL,
		Description:     restaurant.Description,
		WorkingHours:    restaurant.WorkingHours,
		Latitude:        restaurant.Latitude,
		Longitude:       restaurant.Longitude,
		Rating:          restaurant.Rating,
		ReviewCount:     restaurant.ReviewCount,
		Categories:      meta.Categories,
		HasActiveOffers: meta.HasActiveOffers,
	}

	c.JSON(http.StatusOK, response)
}

// @Summary Профиль заведения партнёра
// @Tags Partner
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} RestaurantResponse
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /partner/restaurant [get]
func (h *Handler) GetPartnerRestaurant(c *gin.Context) {
	partnerID, ok := h.requirePartner(c)
	if !ok {
		return
	}

	restaurant, err := h.service.GetPartnerRestaurant(partnerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Restaurant not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch restaurant"})
		return
	}

	metaByRestaurantID, err := h.service.GetOfferMetaByRestaurantIDs([]uuid.UUID{restaurant.ID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to aggregate restaurant metadata"})
		return
	}
	meta := metaByRestaurantID[restaurant.ID]

	c.JSON(http.StatusOK, RestaurantResponse{
		ID:              restaurant.ID.String(),
		Name:            restaurant.Name,
		Address:         restaurant.Address,
		Phone:           restaurant.Phone,
		ImageURL:        restaurant.ImageURL,
		Description:     restaurant.Description,
		WorkingHours:    restaurant.WorkingHours,
		Latitude:        restaurant.Latitude,
		Longitude:       restaurant.Longitude,
		Rating:          restaurant.Rating,
		ReviewCount:     restaurant.ReviewCount,
		Categories:      meta.Categories,
		HasActiveOffers: meta.HasActiveOffers,
	})
}

// @Summary Обновление профиля заведения
// @Tags Partner
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param input body PartnerRestaurantUpdateRequest false "Поля профиля"
// @Success 200 {object} RestaurantResponse
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /partner/restaurant [patch]
func (h *Handler) UpdatePartnerRestaurant(c *gin.Context) {
	partnerID, ok := h.requirePartner(c)
	if !ok {
		return
	}

	var req PartnerRestaurantUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	restaurant, err := h.service.UpdatePartnerRestaurant(partnerID, req)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Restaurant not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update restaurant"})
		return
	}

	metaByRestaurantID, err := h.service.GetOfferMetaByRestaurantIDs([]uuid.UUID{restaurant.ID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to aggregate restaurant metadata"})
		return
	}
	meta := metaByRestaurantID[restaurant.ID]

	c.JSON(http.StatusOK, RestaurantResponse{
		ID:              restaurant.ID.String(),
		Name:            restaurant.Name,
		Address:         restaurant.Address,
		Phone:           restaurant.Phone,
		ImageURL:        restaurant.ImageURL,
		Description:     restaurant.Description,
		WorkingHours:    restaurant.WorkingHours,
		Latitude:        restaurant.Latitude,
		Longitude:       restaurant.Longitude,
		Rating:          restaurant.Rating,
		ReviewCount:     restaurant.ReviewCount,
		Categories:      meta.Categories,
		HasActiveOffers: meta.HasActiveOffers,
	})
}

func calculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371000
	phi1 := lat1 * math.Pi / 180
	phi2 := lat2 * math.Pi / 180
	deltaPhi := (lat2 - lat1) * math.Pi / 180
	deltaLambda := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(deltaPhi/2)*math.Sin(deltaPhi/2) +
		math.Cos(phi1)*math.Cos(phi2)*
			math.Sin(deltaLambda/2)*math.Sin(deltaLambda/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}
