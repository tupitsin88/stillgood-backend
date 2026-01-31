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
		orders.GET("", h.GetUserOrders)
		orders.GET("/:id", h.GetOrderById)
	}
	partner := v1.Group("/partner/orders")
	partner.Use(authMiddleware)
	// Тут можно добавить middleware проверки роли: partner.Use(RoleMiddleware("PARTNER"))
	{
		partner.GET("", h.GetPartnerOrders)
		partner.POST("/:id/complete", h.CompleteOrder)
	}
}
