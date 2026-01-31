package restaurants

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, h *Handler) {
	// Публичные маршруты
	router := r.Group("/api/v1/restaurants")
	{
		router.GET("", h.GetList)
		router.GET("/:id", h.GetByID)
		router.POST("/upload", h.UploadImage)
	}
}
