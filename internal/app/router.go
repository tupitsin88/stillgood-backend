package app

import (
	"kursach_backend/internal/analytics"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "kursach_backend/docs"
	"kursach_backend/internal/auth"
	"kursach_backend/internal/categories"
	"kursach_backend/internal/offers"
	"kursach_backend/internal/orders"
	"kursach_backend/internal/pkg/middleware"
	"kursach_backend/internal/restaurants"
)

func NewRouter(handler *gin.Engine, authService auth.Service, authHandler *auth.Handler, restaurantsHandler *restaurants.Handler, categoriesHandler *categories.Handler, orderHandler *orders.OrderHandler, offerHandler *offers.OfferHandler, analyticsHandler *analytics.AnalyticsHandler, jwtSecret string) {
	// Options
	handler.Use(gin.Recovery())

	// Swagger UI
	handler.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	authMiddleware := middleware.AuthMiddleware(jwtSecret, func(userID string) (bool, error) {
		user, err := authService.GetUserByID(userID)
		if err != nil {
			return false, err
		}
		return user.IsBlocked, nil
	})

	// Init Routes
	auth.RegisterRoutes(handler, authHandler, authMiddleware)
	restaurants.RegisterRoutes(handler, restaurantsHandler, authMiddleware)
	categories.RegisterRoutes(handler, categoriesHandler, authMiddleware)
	orders.RegisterRoutes(handler, orderHandler, authMiddleware)
	offers.RegisterRoutes(handler, offerHandler, authMiddleware)
	analytics.RegisterRoutes(handler, analyticsHandler, authMiddleware)
}
