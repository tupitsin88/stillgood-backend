package auth

import (
	"kursach_backend/internal/pkg/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes реализует маршрутизацию модуля Auth
func RegisterRoutes(r *gin.Engine, h *Handler, authMiddleware gin.HandlerFunc) {
	router := r.Group("/api/v1/auth")
	{
		router.POST("/register", h.Register)
		router.POST("/register/partner", h.RegisterPartner)
		router.POST("/login", h.Login)
		router.POST("/oauth", h.OAuth)
		router.POST("/logout", h.Logout)
		router.POST("/forgot-password", h.ForgotPassword)
		router.POST("/verify-email/request", h.RequestEmailVerification)
		router.POST("/verify-email/confirm", h.VerifyEmail)
		router.POST("/verify-reset-code", h.VerifyResetCode)
		router.POST("/reset-password", h.ResetPassword)
		router.POST("/refresh", h.Refresh)
	}

	protected := r.Group("/api/v1/auth", authMiddleware)
	{
		protected.GET("/me", h.Me)
		protected.POST("/change-password", h.ChangePassword)
	}

	users := r.Group("/api/v1/users", authMiddleware)
	{
		users.PATCH("/me", h.UpdateProfile)
		users.PUT("/me/device-token", h.UpdateDeviceToken)
		users.DELETE("/me", h.DeleteAccount)
	}

	admin := r.Group("/api/v1/admin")
	admin.Use(authMiddleware)
	admin.Use(middleware.RoleMiddleware(RoleAdmin))
	{
		admin.GET("/users", h.GetUsers)
		admin.POST("/users/:id/block", h.BlockUser)
		admin.POST("/users/:id/unblock", h.UnblockUser)
		admin.GET("/partners/pending", h.GetPendingPartners)
		admin.POST("/partners/:id/approve", h.ApprovePartner)
		admin.POST("/partners/:id/reject", h.RejectPartner)
	}
}
