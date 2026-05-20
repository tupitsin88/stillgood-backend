package offers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"kursach_backend/internal/pkg/geo"
	"kursach_backend/internal/pkg/httputil"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const maxOfferUploadSize = 10 * 1024 * 1024

type OfferHandler struct {
	service *OfferService
}

func NewOfferHandler(service *OfferService) *OfferHandler {
	return &OfferHandler{service: service}
}

// CreateOffer @Summary Создание предложения
func (h *OfferHandler) CreateOffer(c *gin.Context) {
	var req CreateOfferRequest
	if !httputil.BindJSON(c, &req) {
		return
	}
	partnerID, ok := partnerIDFromContext(c)
	if !ok {
		return
	}
	offer, err := h.service.CreateOffer(c.Request.Context(), partnerID, req)
	if err != nil {
		switch err.Error() {
		case "RESTAURANT_NOT_FOUND":
			c.JSON(403, gin.H{"error": "PARTNER_NOT_APPROVED", "message": "Partner has no active restaurant"})
		case "INVALID_CATEGORY_ID":
			c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_CATEGORY_ID", "message": "The provided category ID is not a valid UUID"})
		case "INVALID_QUANTITY":
			c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_QUANTITY", "message": "quantity must be a positive total amount"})
		case "INVALID_PRICE":
			c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_PRICE", "message": "price must not exceed originalPrice"})
		case "INVALID_TIME_RANGE":
			c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_TIME_RANGE", "message": "pickupEnd must be after pickupStart"})
		case "START_TIME_IN_PAST":
			c.JSON(http.StatusBadRequest, gin.H{"error": "START_TIME_IN_PAST", "message": "pickupStart must not be in the past"})
		case "PARTNER_NOT_APPROVED":
			c.JSON(http.StatusForbidden, gin.H{"error": "PARTNER_NOT_APPROVED", "message": "Your account must be approved to create offers"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "CREATION_FAILED", "message": "Failed to create offer"})
		}
		return
	}
	dto := h.service.mapToDetailDTO(offer)
	c.JSON(http.StatusCreated, dto)
}

// UpdateOffer @Summary Обновление предложения
func (h *OfferHandler) UpdateOffer(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "INVALID_ID"})
		return
	}
	uidStr := c.GetString("user_id")
	partnerID, err := uuid.Parse(uidStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "UNAUTHORIZED", "message": "Invalid User ID format"})
		return
	}

	var req UpdateOfferRequest
	if !httputil.BindJSON(c, &req) {
		return
	}

	offer, err := h.service.UpdateOffer(c.Request.Context(), id, partnerID, req)
	if err != nil {
		switch err.Error() {
		case "FORBIDDEN":
			c.JSON(403, gin.H{"error": "FORBIDDEN", "message": "You do not own this offer"})
		case "OFFER_NOT_FOUND":
			c.JSON(404, gin.H{"error": "NOT_FOUND", "message": "Offer not found"})
		case "INVALID_CATEGORY_ID":
			c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_CATEGORY_ID", "message": "categoryId must be a valid UUID"})
		case "INVALID_QUANTITY":
			c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_QUANTITY", "message": "quantity cannot be lower than already reserved amount"})
		case "PRICE_MUST_BE_POSITIVE":
			c.JSON(http.StatusBadRequest, gin.H{"error": "PRICE_MUST_BE_POSITIVE", "message": "price must be positive"})
		case "PRICE_TOO_HIGH":
			c.JSON(http.StatusBadRequest, gin.H{"error": "PRICE_TOO_HIGH", "message": "price must not exceed originalPrice"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "UPDATE_FAILED", "message": "Failed to update offer"})
		}
		return
	}

	dto := h.service.mapToDetailDTO(offer)
	c.JSON(200, dto)
}

// GetPartnerOffers @Summary Предложения партнёра
func (h *OfferHandler) GetPartnerOffers(c *gin.Context) {
	partnerID, ok := partnerIDFromContext(c)
	if !ok {
		return
	}
	limit, offset, ok := offerPaginationQuery(c, 20)
	if !ok {
		return
	}

	offers, total, err := h.service.GetPartnerOffers(c.Request.Context(), partnerID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "INTERNAL_ERROR", "message": "Failed to fetch partner offers"})
		return
	}

	c.JSON(200, gin.H{
		"data":       offers,
		"pagination": gin.H{"total": total, "limit": limit, "offset": offset},
	})
}

// GetPublicOffers @Summary Список предложений
func (h *OfferHandler) GetPublicOffers(c *gin.Context) {
	limit, offset, ok := offerPaginationQuery(c, 20)
	if !ok {
		return
	}

	params := FilterParams{
		Limit:  limit,
		Offset: offset,
		SortBy: c.Query("sortBy"),
	}

	if latStr := c.Query("lat"); latStr != "" {
		val, err := strconv.ParseFloat(latStr, 64)
		if err != nil || !geo.ValidLatitude(val) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_LATITUDE", "message": "Latitude must be between -90 and 90"})
			return
		}
		params.Lat = &val
	}

	if lngStr := c.Query("lng"); lngStr != "" {
		val, err := strconv.ParseFloat(lngStr, 64)
		if err != nil || !geo.ValidLongitude(val) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_LONGITUDE", "message": "Longitude must be between -180 and 180"})
			return
		}
		params.Lng = &val
	}

	if activeStr := c.Query("isActive"); activeStr != "" {
		val, err := strconv.ParseBool(activeStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_BOOLEAN", "message": "isActive must be true or false"})
			return
		}
		params.IsActive = &val
	}

	if rStr := c.Query("radius"); rStr != "" {
		val, err := strconv.Atoi(rStr)
		if err != nil || !geo.ValidRadiusMeters(val) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_RADIUS", "message": "Radius must be a positive integer"})
			return
		}
		params.Radius = &val
	}

	if restID := c.Query("restaurantId"); restID != "" {
		val, err := uuid.Parse(restID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_RESTAURANT_ID", "message": "restaurantId must be a valid UUID"})
			return
		}
		params.RestaurantID = &val

	}

	if catID := c.Query("categoryId"); catID != "" {
		val, err := uuid.Parse(catID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_CATEGORY_ID", "message": "categoryId must be a valid UUID"})
			return
		}
		params.CategoryID = &val
	}

	if minP := c.Query("minPrice"); minP != "" {
		val, err := strconv.ParseFloat(minP, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_MIN_PRICE", "message": "minPrice must be a number"})
			return
		}
		params.MinPrice = &val
	}

	if maxP := c.Query("maxPrice"); maxP != "" {
		val, err := strconv.ParseFloat(maxP, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_MAX_PRICE", "message": "maxPrice must be a number"})
			return
		}
		params.MaxPrice = &val
	}

	if params.MinPrice != nil && params.MaxPrice != nil && *params.MinPrice > *params.MaxPrice {
		c.JSON(400, gin.H{"error": "INVALID_PRICE_RANGE", "message": "minPrice cannot be greater than maxPrice"})
		return
	}

	if params.SortBy == "distance" && (params.Lat == nil || params.Lng == nil) {
		c.JSON(400, gin.H{"error": "COORDINATES_REQUIRED", "message": "lat and lng are required for distance sorting"})
		return
	}

	offers, total, err := h.service.GetPublicOffers(c.Request.Context(), params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "INTERNAL_ERROR", "message": "Failed to fetch offers"})
		return
	}

	c.JSON(200, gin.H{
		"data":       offers,
		"pagination": gin.H{"total": total, "limit": limit, "offset": offset},
	})
}

// GetOfferByID @Summary Детали предложения
func (h *OfferHandler) GetOfferByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_ID", "message": "Invalid offer ID format"})
		return
	}

	dto, err := h.service.GetOfferByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(404, gin.H{"error": "NOT_FOUND", "message": "Offer not found"})
		return
	}

	c.JSON(200, dto)
}

// DeleteOffer @Summary Удаление предложения
func (h *OfferHandler) DeleteOffer(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_ID", "message": "Invalid offer ID format"})
		return
	}
	partnerID, ok := partnerIDFromContext(c)
	if !ok {
		return
	}
	if err := h.service.DeleteOffer(c.Request.Context(), id, partnerID); err != nil {
		if err.Error() == "FORBIDDEN" {
			c.JSON(403, gin.H{"error": "FORBIDDEN", "message": "You do not own this offer"})
		} else {
			c.JSON(404, gin.H{"error": "NOT_FOUND", "message": "Offer not found"})
		}
		return
	}
	c.Status(204)
}

// UploadImage @Summary Загрузка изображения для предложения
func (h *OfferHandler) UploadImage(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxOfferUploadSize)
	file, err := c.FormFile("image")
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) || strings.Contains(strings.ToLower(err.Error()), "request body too large") {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "IMAGE_TOO_LARGE", "message": "Max size is 10MB"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_FILE", "message": "No file uploaded"})
		return
	}
	url, err := h.service.UploadImage(file)
	if err != nil {
		switch {
		case errors.Is(err, ErrOfferImageTooLarge):
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "IMAGE_TOO_LARGE", "message": "Max size is 10MB"})
		case errors.Is(err, ErrOfferInvalidImageFormat), errors.Is(err, ErrOfferUnsupportedImageFormat), errors.Is(err, ErrOfferImageProcessingFailed):
			c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_IMAGE_FORMAT", "message": "Unsupported or invalid image format"})
		case errors.Is(err, ErrOfferFileStorageUnavailable):
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "STORAGE_UNAVAILABLE", "message": "File storage is unavailable"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "UPLOAD_FAILED", "message": "Failed to upload image"})
		}
		return
	}
	c.JSON(http.StatusOK, UploadOfferImageResponse{
		URL: url,
	})
}

func partnerIDFromContext(c *gin.Context) (uuid.UUID, bool) {
	partnerID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil || partnerID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "UNAUTHORIZED", "message": "Invalid User ID format"})
		return uuid.Nil, false
	}
	return partnerID, true
}

func offerPaginationQuery(c *gin.Context, defaultLimit int) (int, int, bool) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(defaultLimit)))
	if err != nil || limit <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_LIMIT", "message": "limit must be a positive integer"})
		return 0, 0, false
	}
	if limit > 100 {
		limit = 100
	}

	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil || offset < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_OFFSET", "message": "offset must be a non-negative integer"})
		return 0, 0, false
	}
	return limit, offset, true
}
