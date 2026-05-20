package analytics

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AnalyticsHandler struct {
	service *AnalyticsService
}

func NewAnalyticsHandler(service *AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{service: service}
}

// GetPartnerAnalytics @Summary Аналитика партнёра
func (h *AnalyticsHandler) GetPartnerAnalytics(c *gin.Context) {
	_, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "UNAUTHORIZED"})
		return
	}
	partnerID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil || partnerID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "UNAUTHORIZED", "message": "Invalid User ID format"})
		return
	}
	restaurant, err := h.service.repo.GetRestaurantByPartnerID(c.Request.Context(), partnerID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "RESTAURANT_NOT_FOUND",
			"message": "У вас нет активного ресторана для просмотра аналитики",
		})
		return
	}
	groupBy := c.DefaultQuery("groupBy", "day")
	if groupBy != "day" && groupBy != "week" && groupBy != "month" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_GROUP_BY",
			"message": "Supported values: day, week, month",
		})
		return
	}
	startStr := c.Query("startDate")
	endStr := c.Query("endDate")
	start, errStart := time.Parse("2006-01-02", startStr)
	end, errEnd := time.Parse("2006-01-02", endStr)
	if errStart != nil || errEnd != nil {
		end = time.Now()
		start = end.AddDate(0, 0, -7)
	}

	summary, periods, err := h.service.GetPartnerAnalytics(c.Request.Context(), restaurant.ID, start, end, groupBy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"summary": summary,
		"periods": periods,
	})
}
