package categories

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, h *Handler) {
	router := r.Group("/api/v1/categories")
	{
		router.GET("", h.GetList)
	}
}
