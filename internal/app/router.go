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

func NewRouter(handler *gin.Engine, authHandler *auth.Handler, restaurantsHandler *restaurants.Handler, categoriesHandler *categories.Handler, orderHandler *orders.OrderHandler, offerHandler *offers.OfferHandler, analyticsHandler *analytics.AnalyticsHandler, jwtSecret string) {
	// Options
	//handler.Use(gin.Logger()) // кажется он не нужен и дублирует все логи, поэтому вообще стоит убрать его
	handler.Use(gin.Recovery())

	// Swagger UI
	handler.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Init Routes
	auth.RegisterRoutes(handler, authHandler, middleware.AuthMiddleware(jwtSecret))
	restaurants.RegisterRoutes(handler, restaurantsHandler, middleware.AuthMiddleware(jwtSecret))
	categories.RegisterRoutes(handler, categoriesHandler)
	orders.RegisterRoutes(handler, orderHandler, middleware.AuthMiddleware(jwtSecret))
	offers.RegisterRoutes(handler, offerHandler, middleware.AuthMiddleware(jwtSecret))
	analytics.RegisterRoutes(handler, analyticsHandler, middleware.AuthMiddleware(jwtSecret))
}
