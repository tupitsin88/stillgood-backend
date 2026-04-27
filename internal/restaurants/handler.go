package restaurants

import (
	"errors"
	"kursach_backend/internal/auth"
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

// CreateRestaurant @Summary Создание карточки ресторана
// @Tags Restaurants
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param input body CreateRestaurantRequest true "Данные ресторана"
// @Success 201 {object} RestaurantResponse
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /restaurants [post]
func (h *Handler) CreateRestaurant(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "UNAUTHORIZED"})
		return
	}

	var req CreateRestaurantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.CompanyName = strings.TrimSpace(req.CompanyName)
	req.Inn = strings.TrimSpace(req.Inn)
	req.Address = strings.TrimSpace(req.Address)
	if req.Name == "" || req.CompanyName == "" || req.Inn == "" || req.Address == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name, companyName, inn and address are required"})
		return
	}

	var partnerID string
	switch c.GetString("role") {
	case auth.RoleAdmin:
		if req.PartnerID == nil || strings.TrimSpace(*req.PartnerID) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "partnerId is required for admin"})
			return
		}
		partnerID = strings.TrimSpace(*req.PartnerID)
	case auth.RolePartner:
		if req.PartnerID != nil && strings.TrimSpace(*req.PartnerID) != "" && strings.TrimSpace(*req.PartnerID) != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "FORBIDDEN", "message": "Partner can create restaurant only for self"})
			return
		}
		partnerID = userID
	default:
		c.JSON(http.StatusForbidden, gin.H{"error": "FORBIDDEN", "message": "Admin or approved partner role required"})
		return
	}

	restaurant, err := h.service.CreateRestaurant(partnerID, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrPartnerNotApproved):
			c.JSON(http.StatusForbidden, gin.H{"error": "PARTNER_NOT_APPROVED", "message": "Partner is not approved"})
		case errors.Is(err, ErrRestaurantAlreadyExists):
			c.JSON(http.StatusConflict, gin.H{"error": "RESTAURANT_ALREADY_EXISTS"})
		case errors.Is(err, gorm.ErrRecordNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "Partner not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create restaurant"})
		}
		return
	}

	c.JSON(http.StatusCreated, RestaurantResponse{
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
		HasActiveOffers: false,
		Categories:      []string{},
	})
}

func (h *Handler) requirePartner(c *gin.Context) (string, bool) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "UNAUTHORIZED"})
		return "", false
	}
	if c.GetString("role") != auth.RolePartner {
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

// UploadImage @Summary Загрузка изображения
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

// GetList @Summary Список ресторанов
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

// GetByID @Summary Детали ресторана
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

// GetPartnerRestaurant @Summary Профиль заведения партнёра
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

// UpdatePartnerRestaurant @Summary Обновление профиля заведения
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

// UpdateAdminRestaurant @Summary Обновление административных полей ресторана
// @Tags Admin
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path string true "Restaurant ID"
// @Param input body AdminRestaurantUpdateRequest true "Административные поля"
// @Success 200 {object} AdminRestaurantResponse
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /admin/restaurants/{id} [patch]
func (h *Handler) UpdateAdminRestaurant(c *gin.Context) {
	var req AdminRestaurantUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	restaurant, err := h.service.UpdateAdminRestaurant(c.Param("id"), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidRestaurantID):
			c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_RESTAURANT_ID"})
		case errors.Is(err, ErrInvalidCommission):
			c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_COMMISSION", "message": "commission must be between 0 and 100"})
		case errors.Is(err, gorm.ErrRecordNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "Restaurant not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update restaurant"})
		}
		return
	}

	c.JSON(http.StatusOK, AdminRestaurantResponse{
		ID:         restaurant.ID.String(),
		Name:       restaurant.Name,
		Commission: restaurant.Commission,
		IsActive:   restaurant.IsActive,
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

// GetReviews @Summary Получить отзывы ресторана
// @Tags Restaurants
// @Produce json
// @Param id path string true "Restaurant ID"
// @Param limit query int false "Лимит" default(10)
// @Param offset query int false "Сдвиг" default(0)
// @Success 200 {object} RestaurantReviewsResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /restaurants/{id}/reviews [get]
func (h *Handler) GetReviews(c *gin.Context) {
	restID := c.Param("id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	reviews, total, err := h.service.GetReviews(restID, limit, offset)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to fetch reviews"})
		return
	}

	data := make([]ReviewDTO, len(reviews))
	for i, r := range reviews {
		data[i] = ReviewDTO{
			ID:        r.ID.String(),
			Rating:    r.Rating,
			Comment:   r.Comment,
			CreatedAt: r.CreatedAt,
		}
	}

	c.JSON(http.StatusOK, RestaurantReviewsResponse{
		Data: data,
		Pagination: Pagination{
			Total:  total,
			Limit:  limit,
			Offset: offset,
		},
	})
}

// DeleteReview @Summary Удалить отзыв (Admin)
// @Tags Admin
// @Security ApiKeyAuth
// @Param id path string true "Review ID"
// @Success 204 "No Content"
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/reviews/{id} [delete]
func (h *Handler) DeleteReview(c *gin.Context) {
	reviewID := c.Param("id")
	if err := h.service.DeleteReview(reviewID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete review"})
		return
	}
	c.Status(http.StatusNoContent)
}

// GetAdminReviews @Summary Список отзывов с данными авторов (Admin)
// @Tags Admin
// @Security ApiKeyAuth
// @Produce json
// @Param id path string true "Restaurant ID"
// @Success 200 {object} []AdminReviewDTO
// @Router /admin/restaurants/{id}/reviews [get]
func (h *Handler) GetAdminReviews(c *gin.Context) {
	restID := c.Param("id")
	reviews, _, err := h.service.GetReviews(restID, 100, 0)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to fetch reviews"})
		return
	}
	var data []AdminReviewDTO
	for _, r := range reviews {
		data = append(data, AdminReviewDTO{
			ID:        r.ID.String(),
			Rating:    r.Rating,
			Comment:   r.Comment,
			UserName:  r.User.Name,
			UserEmail: r.User.Email,
			CreatedAt: r.CreatedAt,
		})
	}
	c.JSON(200, data)
}

// GetPartnerReviews @Summary Отзывы моего ресторана (Partner)
// @Tags Partner
// @Security ApiKeyAuth
// @Produce json
// @Router /partner/reviews [get]
func (h *Handler) GetPartnerReviews(c *gin.Context) {
	userID := c.GetString("user_id")
	rest, err := h.service.GetPartnerRestaurant(userID)
	if err != nil {
		c.JSON(404, gin.H{"error": "RESTAURANT_NOT_FOUND"})
		return
	}
	reviews, total, err := h.service.GetReviews(rest.ID.String(), 10, 0)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to fetch reviews"})
		return
	}
	var data []ReviewDTO
	for _, r := range reviews {
		data = append(data, ReviewDTO{
			ID:        r.ID.String(),
			Rating:    r.Rating,
			Comment:   r.Comment,
			CreatedAt: r.CreatedAt,
		})
	}
	c.JSON(200, gin.H{
		"data":       data,
		"pagination": gin.H{"total": total, "limit": 10, "offset": 0},
	})
}
