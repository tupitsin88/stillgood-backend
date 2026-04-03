package analytics

import (
	"fmt"
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
// @Description Получение статистики продаж, выручки и процента отмен за период с группировкой
// @Tags Analytics
// @Security ApiKeyAuth
// @Param startDate query string true "Дата начала (YYYY-MM-DD)"
// @Param endDate query string true "Дата конца (YYYY-MM-DD)"
// @Param groupBy query string false "Группировка данных" Enums(day, week, month) default(day) // <-- ДОБАВЛЯЕМ ЭТУ СТРОКУ
// @Success 200 {object} AnalyticsSummary
// @Router /partner/analytics [get]
func (h *AnalyticsHandler) GetPartnerAnalytics(c *gin.Context) {
	uidValue, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "UNAUTHORIZED"})
		return
	}
	partnerID, _ := uuid.Parse(fmt.Sprintf("%v", uidValue))
	restaurant, err := h.service.repo.GetRestaurantByPartnerID(c.Request.Context(), partnerID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "RESTAURANT_NOT_FOUND",
			"message": "У вас нет активного ресторана для просмотра аналитики",
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
	groupBy := c.DefaultQuery("groupBy", "day")
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
