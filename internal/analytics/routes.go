package analytics

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.Engine, h *AnalyticsHandler, authMiddleware gin.HandlerFunc) {
	analytics := router.Group("/api/v1/partner/analytics")
	analytics.Use(authMiddleware)
	{
		analytics.GET("", h.GetPartnerAnalytics)
	}
}
