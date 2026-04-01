package auth

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes реализует маршрутизацию модуля Auth
func RegisterRoutes(r *gin.Engine, h *Handler, middleware gin.HandlerFunc) {
	router := r.Group("/api/v1/auth")
	{
		router.POST("/register", h.Register)
		router.POST("/register/partner", h.RegisterPartner)
		router.POST("/login", h.Login)
		router.POST("/refresh", h.Refresh)
	}

	protected := r.Group("/api/v1/auth", middleware)
	{
		protected.GET("/me", h.Me)
		protected.POST("/change-password", h.ChangePassword)
		protected.POST("/logout", h.Logout)
	}
}
