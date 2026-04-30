package restaurants

import (
	"kursach_backend/internal/auth"
	"kursach_backend/internal/pkg/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, h *Handler, authMiddleware gin.HandlerFunc) {
	v1 := r.Group("/api/v1")
	res := v1.Group("/restaurants")
	{
		res.GET("", h.GetList)
		res.GET("/:id", h.GetByID)
		res.GET("/:id/reviews", h.GetReviews)
		res.POST("/upload", h.UploadImage)
		res.Use(authMiddleware)
		{
			res.POST("", h.CreateRestaurant)
		}
	}
	admin := v1.Group("/admin")
	admin.Use(authMiddleware, middleware.RoleMiddleware(auth.RoleAdmin))
	{
		admin.PATCH("/restaurants/:id", h.UpdateAdminRestaurant)
		admin.GET("/reviews", h.GetAdminReviewList)
		admin.GET("/restaurants/:id/reviews", h.GetAdminReviews)
		admin.DELETE("/reviews/:id", h.DeleteReview)
	}
	partner := v1.Group("/partner")
	partner.Use(authMiddleware)
	{
		partner.GET("/restaurant", h.GetPartnerRestaurant)
		partner.PATCH("/restaurant", h.UpdatePartnerRestaurant)
		partner.GET("/reviews", h.GetPartnerReviews)
	}
}
