package main

import (
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"kursach_backend/internal/app"
	"kursach_backend/internal/auth"
	"kursach_backend/internal/categories"
	"kursach_backend/internal/domain"
	"kursach_backend/internal/pkg/filestorage"
	"kursach_backend/internal/restaurants"
	"kursach_backend/pkg/postgres"
)

// @title FoodSharing App API
// @version 1.0
// @description API сервер для курсовой работы FoodSharing.
// @host localhost:8080
// @BasePath /api/v1
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization
func main() {
	// 1. Config
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "host=localhost user=postgres password=password dbname=foodsharing_db port=5433 sslmode=disable"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "supersecretkey"
	}

	// 2. Database
	db, err := postgres.NewDB(dsn)
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	// Migrations
	if err := db.AutoMigrate(&domain.User{}, &domain.Restaurant{}, &domain.Category{}); err != nil {
		log.Fatalf("failed to migrate database: %v", err)
	}

	// 3. Dependencies
	tokenManager, err := auth.NewTokenManager(jwtSecret)
	if err != nil {
		log.Fatalf("failed to init token manager: %v", err)
	}

	authRepo := auth.NewRepository(db)
	authService := auth.NewService(authRepo, tokenManager, 30*time.Minute, 14*24*time.Hour)
	authHandler := auth.NewHandler(authService)

	// File Storage
	minioEndpoint := "localhost:9000"
	minioAccessKey := "minioadmin"
	minioSecretKey := "minioadmin"
	minioBucket := "food-images"
	minioUseSSL := false

	fileStorage, err := filestorage.NewFileStorage(minioEndpoint, minioAccessKey, minioSecretKey, minioBucket, minioUseSSL)
	if err != nil {
		log.Fatalf("failed to init file storage: %v", err)
	}
	log.Printf("File storage initialized for endpoint: %s", minioEndpoint)
	_ = fileStorage // Will be used in future handlers

	restaurantsRepo := restaurants.NewRepository(db)
	restaurantsService := restaurants.NewService(restaurantsRepo, fileStorage)
	restaurantsHandler := restaurants.NewHandler(restaurantsService)

	categoriesRepo := categories.NewRepository(db)
	categoriesService := categories.NewService(categoriesRepo)
	categoriesHandler := categories.NewHandler(categoriesService)

	// 4. Router
	router := gin.Default()
	app.NewRouter(router, authHandler, restaurantsHandler, categoriesHandler, jwtSecret)

	// 5. Run
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
