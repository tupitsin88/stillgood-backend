package offers

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type OfferHandler struct {
	service *OfferService
}

func NewOfferHandler(service *OfferService) *OfferHandler {
	return &OfferHandler{service: service}
}

// CreateOffer godoc
// @Tags offers
// @Security ApiKeyAuth
// @Summary Create a new offer
// @Accept json
// @Produce json
// @Param request body CreateOfferRequest true "Create Offer Request"
// @Success 201 {object} offers.OfferDetailDTO
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/partner/offers [post]
func (h *OfferHandler) CreateOffer(c *gin.Context) {
	var req CreateOfferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "INVALID_REQUEST", "message": err.Error()})
		return
	}

	uidStr := c.GetString("user_id")
	partnerID, _ := uuid.Parse(uidStr)

	offer, err := h.service.CreateOffer(c.Request.Context(), partnerID, req)
	if err != nil {
		if err.Error() == "RESTAURANT_NOT_FOUND" {
			c.JSON(403, gin.H{"error": "PARTNER_NOT_APPROVED", "message": "Partner has no active restaurant"})
			return
		}
		c.JSON(400, gin.H{"error": "CREATION_FAILED", "message": err.Error()})
		return
	}

	dto := h.service.mapToDetailDTO(offer)
	c.JSON(201, dto)
}

// UpdateOffer godoc
// @Tags offers
// @Security ApiKeyAuth
// @Summary Update an existing offer
// @Accept json
// @Produce json
// @Param id path string true "Offer ID"
// @Param request body UpdateOfferRequest true "Update Offer Request"
// @Success 200 {object} offers.OfferDetailDTO
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/partner/offers/{id} [patch]
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

// GetPartnerOffers godoc
// @Tags offers
// @Security ApiKeyAuth
// @Summary Get partner's offers
// @Accept json
// @Produce json
// @Success 200 {object} offers.GetPartnerOffersResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/partner/offers [get]
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

// GetPublicOffers godoc
// @Tags offers
// @Summary Get public offers list
// @Accept json
// @Produce json
// @Param lat query number false "Latitude"
// @Param lng query number false "Longitude"
// @Param radius query int false "Radius (meters)"
// @Param price_min query number false "Min Price"
// @Param price_max query number false "Max Price"
// @Param category_id query string false "Category ID"
// @Param sortBy query string false "Sort By"
// @Param limit query int false "Limit"
// @Param offset query int false "Offset"
// @Success 200 {object} offers.GetOffersResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/offers [get]
func (h *OfferHandler) GetPublicOffers(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	params := FilterParams{
		Limit:  limit,
		Offset: offset,
		SortBy: c.Query("sortBy"),
	}

	if latStr := c.Query("lat"); latStr != "" {
		if val, err := strconv.ParseFloat(latStr, 64); err == nil {
			params.Lat = &val
		}
	}
	if lngStr := c.Query("lng"); lngStr != "" {
		if val, err := strconv.ParseFloat(lngStr, 64); err == nil {
			params.Lng = &val
		}
	}
	if rStr := c.Query("radius"); rStr != "" {
		if val, err := strconv.Atoi(rStr); err == nil {
			params.Radius = &val
		}
	}

	if restID := c.Query("restaurantId"); restID != "" {
		if val, err := uuid.Parse(restID); err == nil {
			params.RestaurantID = &val
		}
	}

	if catID := c.Query("categoryId"); catID != "" {
		if val, err := uuid.Parse(catID); err == nil {
			params.CategoryID = &val
		}
	}

	if minP := c.Query("minPrice"); minP != "" {
		if val, err := strconv.ParseFloat(minP, 64); err == nil {
			params.MinPrice = &val
		}
	}
	if maxP := c.Query("maxPrice"); maxP != "" {
		if val, err := strconv.ParseFloat(maxP, 64); err == nil {
			params.MaxPrice = &val
		}
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

// GetOfferByID godoc
// @Tags offers
// @Summary Get offer details by ID
// @Accept json
// @Produce json
// @Param id path string true "Offer ID"
// @Success 200 {object} offers.OfferDetailDTO
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/offers/{id} [get]
func (h *OfferHandler) GetOfferByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))

	dto, err := h.service.GetOfferByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(404, gin.H{"error": "NOT_FOUND", "message": "Offer not found"})
		return
	}

	c.JSON(200, dto)
}
