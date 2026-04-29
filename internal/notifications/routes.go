package notifications

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, h *NotificationsHandler, authMiddleware gin.HandlerFunc) {
	api := r.Group("/api/v1/notifications")
	api.Use(authMiddleware)
	{
		api.GET("", h.GetMyNotifications)
	}
}
