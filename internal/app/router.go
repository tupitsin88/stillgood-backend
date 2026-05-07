package app

import (
	"net/http"

	"kursach_backend/internal/adminui"
	"kursach_backend/internal/analytics"
	"kursach_backend/internal/notifications"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"kursach_backend/internal/auth"
	"kursach_backend/internal/categories"
	"kursach_backend/internal/offers"
	"kursach_backend/internal/orders"
	"kursach_backend/internal/pkg/middleware"
	"kursach_backend/internal/restaurants"
)

func NewRouter(handler *gin.Engine, authService auth.Service, authHandler *auth.Handler, restaurantsHandler *restaurants.Handler, categoriesHandler *categories.Handler, orderHandler *orders.OrderHandler, offerHandler *offers.OfferHandler, analyticsHandler *analytics.AnalyticsHandler, notificationsHandler *notifications.NotificationsHandler, adminHandler *adminui.Handler, jwtSecret string) {
	handler.Use(gin.Recovery())
	RegisterHealthRoutes(handler)

	// Swagger UI
	handler.StaticFile("/openapi.yaml", "docs/openapi.yaml")
	handler.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/openapi.yaml")))

	authMiddleware := middleware.AuthMiddleware(jwtSecret, func(userID string) (bool, error) {
		return authService.IsUserBlocked(userID)
	})

	// Init Routes
	adminui.RegisterRoutes(handler, adminHandler)
	auth.RegisterRoutes(handler, authHandler, authMiddleware)
	restaurants.RegisterRoutes(handler, restaurantsHandler, authMiddleware)
	categories.RegisterRoutes(handler, categoriesHandler, authMiddleware)
	orders.RegisterRoutes(handler, orderHandler, authMiddleware)
	offers.RegisterRoutes(handler, offerHandler, authMiddleware)
	analytics.RegisterRoutes(handler, analyticsHandler, authMiddleware)
	notifications.RegisterRoutes(handler, notificationsHandler, authMiddleware)
}

func RegisterHealthRoutes(handler *gin.Engine) {
	handler.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
}
