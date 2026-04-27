package orders

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, h *OrderHandler, authMiddleware gin.HandlerFunc) {
	v1 := r.Group("/api/v1")
	orders := v1.Group("/orders")
	orders.Use(authMiddleware)
	{
		orders.POST("", h.CreateOrder)
		orders.POST("/:id/pay", h.PayOrder)
		orders.POST("/:id/cancel", h.CancelOrder)
		orders.POST("/:id/review", h.CreateReview)
		orders.GET("", h.GetUserOrders)
		orders.GET("/:id", h.GetOrderById)
		orders.GET("/me/stats", h.GetUserStats)
		orders.GET("/me/notifications", h.GetNotifications)
	}
	partner := v1.Group("/partner/orders")
	partner.Use(authMiddleware)
	partner.Use(func(c *gin.Context) {
		role := c.GetString("role")
		if role != "PARTNER" {
			c.AbortWithStatusJSON(403, gin.H{"error": "FORBIDDEN", "message": "Only partners allowed"})
			return
		}
		c.Next()
	})
	{
		partner.GET("", h.GetPartnerOrders)
		partner.POST("/:id/complete", h.CompleteOrder)
	}
}
