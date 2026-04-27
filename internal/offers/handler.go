package offers

import (
	"fmt"
	"net/http"
	"strconv"

	"kursach_backend/internal/pkg/geo"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type OfferHandler struct {
	service *OfferService
}

func NewOfferHandler(service *OfferService) *OfferHandler {
	return &OfferHandler{service: service}
}

// CreateOffer @Summary Создание предложения
// @Tags Partner
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param input body CreateOfferRequest true "Данные предложения"
// @Success 201 {object} OfferDetailDTO
// @Router /partner/offers [post]
func (h *OfferHandler) CreateOffer(c *gin.Context) {
	var req CreateOfferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_REQUEST", "message": err.Error()})
		return
	}
	uidValue, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "UNAUTHORIZED", "message": "User ID not found in context"})
		return
	}
	uidStr := fmt.Sprintf("%v", uidValue)
	partnerID, err := uuid.Parse(uidStr)
	if err != nil || partnerID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "UNAUTHORIZED", "message": "Invalid User ID format"})
		return
	}
	offer, err := h.service.CreateOffer(c.Request.Context(), partnerID, req)
	if err != nil {
		switch err.Error() {
		case "RESTAURANT_NOT_FOUND":
			c.JSON(403, gin.H{"error": "PARTNER_NOT_APPROVED", "message": "Partner has no active restaurant"})
		case "INVALID_CATEGORY_ID":
			c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_CATEGORY_ID", "message": "The provided category ID is not a valid UUID"})
		default:
			c.JSON(400, gin.H{"error": "CREATION_FAILED", "message": err.Error()})
		}
		return
	}
	dto := h.service.mapToDetailDTO(offer)
	c.JSON(http.StatusCreated, dto)
}

// UpdateOffer @Summary Обновление предложения
// @Tags Partner
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path string true "Offer ID"
// @Param input body UpdateOfferRequest true "Данные для обновления"
// @Success 200 {object} OfferDetailDTO
// @Failure 403 {object} map[string]string
// @Router /partner/offers/{id} [patch]
func (h *OfferHandler) UpdateOffer(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	uidStr := c.GetString("user_id")
	partnerID, _ := uuid.Parse(uidStr)

	var req UpdateOfferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "INVALID_REQUEST"})
		return
	}

	offer, err := h.service.UpdateOffer(c.Request.Context(), id, partnerID, req)
	if err != nil {
		switch err.Error() {
		case "FORBIDDEN":
			c.JSON(403, gin.H{"error": "FORBIDDEN", "message": "You do not own this offer"})
		case "OFFER_NOT_FOUND":
			c.JSON(404, gin.H{"error": "NOT_FOUND", "message": "Offer not found"})
		default:
			c.JSON(400, gin.H{"error": "UPDATE_FAILED", "message": err.Error()})
		}
		return
	}

	dto := h.service.mapToDetailDTO(offer)
	c.JSON(200, dto)
}

// GetPartnerOffers @Summary Предложения партнёра
// @Tags Partner
// @Security ApiKeyAuth
// @Produce json
// @Param limit query integer false "Limit"
// @Param offset query integer false "Offset"
// @Success 200 {object} map[string]interface{}
// @Router /partner/offers [get]
func (h *OfferHandler) GetPartnerOffers(c *gin.Context) {
	uidStr := c.GetString("user_id")
	partnerID, _ := uuid.Parse(uidStr)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	offers, total, err := h.service.GetPartnerOffers(c.Request.Context(), partnerID, limit, offset)
	if err != nil {
		c.JSON(500, gin.H{"error": "INTERNAL_ERROR", "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"data":       offers,
		"pagination": gin.H{"total": total, "limit": limit, "offset": offset},
	})
}

// GetPublicOffers @Summary Список предложений
// @Tags Offers
// @Produce json
// @Param lat query number false "Latitude"
// @Param lng query number false "Longitude"
// @Param radius query integer false "Radius"
// @Param restaurantId query string false "Restaurant ID"
// @Param categoryId query string false "Category ID"
// @Param minPrice query number false "Min Price"
// @Param maxPrice query number false "Max Price"
// @Param sortBy query string false "Sort By" Enums(distance, price, pickupTime)
// @Param limit query integer false "Limit"
// @Param offset query integer false "Offset"
// @Success 200 {object} map[string]interface{}
// @Router /offers [get]
func (h *OfferHandler) GetPublicOffers(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

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

	offers, total, err := h.service.GetPublicOffers(c.Request.Context(), params)
	if err != nil {
		c.JSON(500, gin.H{"error": "INTERNAL_ERROR", "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"data":       offers,
		"pagination": gin.H{"total": total, "limit": limit, "offset": offset},
	})
}

// GetOfferByID @Summary Детали предложения
// @Tags Offers
// @Produce json
// @Param id path string true "Offer ID"
// @Success 200 {object} OfferDetailDTO
// @Failure 404 {object} map[string]string
// @Router /offers/{id} [get]
func (h *OfferHandler) GetOfferByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))

	dto, err := h.service.GetOfferByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(404, gin.H{"error": "NOT_FOUND", "message": "Offer not found"})
		return
	}

	c.JSON(200, dto)
}

// DeleteOffer @Summary Удаление предложения
// @Tags Partner
// @Security ApiKeyAuth
// @Param id path string true "Offer ID"
// @Success 204 "No Content"
// @Failure 403 {object} map[string]string
// @Router /partner/offers/{id} [delete]
func (h *OfferHandler) DeleteOffer(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_ID", "message": "Invalid offer ID format"})
		return
	}
	uidStr := c.GetString("user_id")
	partnerID, _ := uuid.Parse(uidStr)
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
