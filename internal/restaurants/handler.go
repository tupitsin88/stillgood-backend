package restaurants

import (
	"errors"
	"kursach_backend/internal/auth"
	"kursach_backend/internal/domain"
	"kursach_backend/internal/pkg/geo"
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
	if !geo.ValidLatitude(req.Latitude) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid latitude"})
		return
	}
	if !geo.ValidLongitude(req.Longitude) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid longitude"})
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
		case errors.Is(err, ErrInvalidCoordinates):
			c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_COORDINATES"})
		case errors.Is(err, ErrInvalidPhone):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid phone", "message": "phone must be a valid E.164 phone number"})
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
		LogoURL:         restaurant.LogoURL,
		CoverURL:        restaurant.CoverURL,
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
func (h *Handler) UploadImage(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadRequestBodyBytes)
	kind := strings.ToLower(strings.TrimSpace(c.PostForm("kind")))

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

	url, err := h.service.UploadImage(file, kind)
	if err != nil {
		if errors.Is(err, ErrInvalidImageKind) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid image kind", "message": "kind must be logo or cover"})
			return
		}
		if errors.Is(err, ErrInvalidImageFormat) || errors.Is(err, ErrImageProcessingFailed) {
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

	field := "logoUrl"
	if kind == "cover" {
		field = "coverUrl"
	}
	c.JSON(http.StatusOK, UploadRestaurantImageResponse{
		Kind:  kind,
		Field: field,
		URL:   url,
	})
}

// GetList @Summary Список ресторанов
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
		if parseErr != nil || !geo.ValidLatitude(lat) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid lat"})
			return
		}
		params.Lat = &lat
	}

	if lngStr := c.Query("lng"); lngStr != "" {
		lng, parseErr := strconv.ParseFloat(lngStr, 64)
		if parseErr != nil || !geo.ValidLongitude(lng) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid lng"})
			return
		}
		params.Lng = &lng
	}

	if radiusStr := c.Query("radius"); radiusStr != "" {
		radius, parseErr := strconv.Atoi(radiusStr)
		if parseErr != nil || !geo.ValidRadiusMeters(radius) {
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
		meta := metaByRestaurantID[r.ID]
		response = append(response, RestaurantResponse{
			ID:              r.ID.String(),
			Name:            r.Name,
			Address:         r.Address,
			Phone:           r.Phone,
			LogoURL:         r.LogoURL,
			CoverURL:        r.CoverURL,
			Description:     r.Description,
			WorkingHours:    r.WorkingHours,
			Latitude:        r.Latitude,
			Longitude:       r.Longitude,
			Rating:          r.Rating,
			ReviewCount:     r.ReviewCount,
			Categories:      meta.Categories,
			HasActiveOffers: meta.HasActiveOffers,
			Distance:        r.DistanceMeters,
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
		LogoURL:         restaurant.LogoURL,
		CoverURL:        restaurant.CoverURL,
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
		LogoURL:         restaurant.LogoURL,
		CoverURL:        restaurant.CoverURL,
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
		if errors.Is(err, ErrInvalidPhone) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid phone", "message": "phone must be a valid E.164 phone number"})
			return
		}
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
		LogoURL:         restaurant.LogoURL,
		CoverURL:        restaurant.CoverURL,
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

// GetReviews @Summary Получить отзывы ресторана
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
func (h *Handler) DeleteReview(c *gin.Context) {
	reviewID := c.Param("id")
	if err := h.service.DeleteReview(reviewID); err != nil {
		switch {
		case errors.Is(err, ErrInvalidReviewID):
			c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_REVIEW_ID"})
		case errors.Is(err, gorm.ErrRecordNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "REVIEW_NOT_FOUND"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete review"})
		}
		return
	}
	c.Status(http.StatusNoContent)
}

// GetAdminReviewList @Summary Общий список отзывов (Admin)
func (h *Handler) GetAdminReviewList(c *gin.Context) {
	limit, offset, ok := adminPaginationQuery(c, 20)
	if !ok {
		return
	}

	var restaurantID *uuid.UUID
	if restaurantIDStr := strings.TrimSpace(c.Query("restaurantId")); restaurantIDStr != "" {
		parsed, err := uuid.Parse(restaurantIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_RESTAURANT_ID"})
			return
		}
		restaurantID = &parsed
	}

	reviews, total, err := h.service.GetAdminReviews(restaurantID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch reviews"})
		return
	}

	c.JSON(http.StatusOK, AdminReviewsResponse{
		Data:       adminReviewDTOs(reviews),
		Pagination: Pagination{Total: total, Limit: limit, Offset: offset},
	})
}

// GetAdminReviews @Summary Список отзывов с данными авторов (Admin)
func (h *Handler) GetAdminReviews(c *gin.Context) {
	restaurantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_RESTAURANT_ID"})
		return
	}

	limit, offset, ok := adminPaginationQuery(c, 20)
	if !ok {
		return
	}

	reviews, total, err := h.service.GetAdminReviews(&restaurantID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch reviews"})
		return
	}
	c.JSON(http.StatusOK, AdminReviewsResponse{
		Data:       adminReviewDTOs(reviews),
		Pagination: Pagination{Total: total, Limit: limit, Offset: offset},
	})
}

func adminReviewDTOs(reviews []domain.Review) []AdminReviewDTO {
	data := make([]AdminReviewDTO, 0, len(reviews))
	for _, r := range reviews {
		data = append(data, AdminReviewDTO{
			ID:           r.ID.String(),
			RestaurantID: r.RestaurantID.String(),
			Rating:       r.Rating,
			Comment:      r.Comment,
			UserName:     r.User.Name,
			UserEmail:    r.User.Email,
			CreatedAt:    r.CreatedAt,
		})
	}
	return data
}

func adminPaginationQuery(c *gin.Context, defaultLimit int) (int, int, bool) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(defaultLimit)))
	if err != nil || limit <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid limit"})
		return 0, 0, false
	}
	if limit > 100 {
		limit = 100
	}

	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil || offset < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid offset"})
		return 0, 0, false
	}
	return limit, offset, true
}

// GetPartnerReviews @Summary Отзывы моего ресторана (Partner)
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
