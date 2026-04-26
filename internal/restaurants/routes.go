package restaurants

import (
	"kursach_backend/internal/auth"
	"kursach_backend/internal/pkg/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, h *Handler, authMiddleware gin.HandlerFunc) {
	router := r.Group("/api/v1/restaurants")
	{
		router.GET("", h.GetList)
		router.GET("/:id", h.GetByID)
		router.POST("/upload", h.UploadImage)
	}

	protected := r.Group("/api/v1/restaurants")
	protected.Use(authMiddleware)
	{
		protected.POST("", h.CreateRestaurant)
	}

	admin := r.Group("/api/v1/admin/restaurants")
	admin.Use(authMiddleware)
	admin.Use(middleware.RoleMiddleware(auth.RoleAdmin))
	{
		admin.PATCH("/:id", h.UpdateAdminRestaurant)
	}

	partner := r.Group("/api/v1/partner")
	partner.Use(authMiddleware)
	{
		partner.GET("/restaurant", h.GetPartnerRestaurant)
		partner.PATCH("/restaurant", h.UpdatePartnerRestaurant)
	}
}
