package restaurants

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, h *Handler, authMiddleware gin.HandlerFunc) {
	// Публичные маршруты
	router := r.Group("/api/v1/restaurants")
	{
		router.GET("", h.GetList)
		router.GET("/:id", h.GetByID)
		router.POST("/upload", h.UploadImage)
	}

	partner := r.Group("/api/v1/partner")
	partner.Use(authMiddleware)
	{
		partner.GET("/restaurant", h.GetPartnerRestaurant)
		partner.PATCH("/restaurant", h.UpdatePartnerRestaurant)
	}
}
