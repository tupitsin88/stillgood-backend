package app

import (
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

func NewRouter(handler *gin.Engine, authHandler *auth.Handler, restaurantsHandler *restaurants.Handler, categoriesHandler *categories.Handler, orderHandler *orders.OrderHandler, offerHandler *offers.OfferHandler, jwtSecret string) {
	// Options
	handler.Use(gin.Logger())
	handler.Use(gin.Recovery())

	// Swagger UI
	handler.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Init Routes
	auth.RegisterRoutes(handler, authHandler, middleware.AuthMiddleware(jwtSecret))
	restaurants.RegisterRoutes(handler, restaurantsHandler)
	categories.RegisterRoutes(handler, categoriesHandler)
	orders.RegisterRoutes(handler, orderHandler, middleware.AuthMiddleware(jwtSecret))
	offers.RegisterRoutes(handler, offerHandler, middleware.AuthMiddleware(jwtSecret))
}
