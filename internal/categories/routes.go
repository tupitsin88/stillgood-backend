package categories

import (
	"kursach_backend/internal/pkg/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, h *Handler, authMiddleware gin.HandlerFunc) {
	router := r.Group("/api/v1/categories")
	{
		router.GET("", h.GetList)
		admin := router.Group("")
		admin.Use(authMiddleware)
		admin.Use(middleware.RoleMiddleware("ADMIN"))
		{
			admin.POST("", h.Create)
			admin.PATCH("/:id", h.Update)
			admin.DELETE("/:id", h.Delete)
		}
	}
}
