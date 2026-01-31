package main

import (
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"kursach_backend/internal/domain"
	"kursach_backend/internal/offers"

	// "kursach_backend/internal/offers"
	"kursach_backend/internal/orders"

	"kursach_backend/internal/app"
	"kursach_backend/internal/auth"
	"kursach_backend/internal/categories"
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
	dsn := "host=" + os.Getenv("DB_HOST") +
		" user=" + os.Getenv("DB_USER") +
		" password=" + os.Getenv("DB_PASSWORD") +
		" dbname=" + os.Getenv("DB_NAME") +
		" port=" + os.Getenv("DB_PORT") +
		" sslmode=disable"
	db, err := postgres.NewDB(dsn)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "supersecretkey"
	}

	// 2. Авто-миграции
	log.Println("Running migrations...")
	err = db.AutoMigrate(
		&domain.User{},
		&domain.Restaurant{},
		&domain.Offer{},
		&domain.Order{},
		&domain.OrderStatusHistory{},
		&domain.Category{},
	)
	if err != nil {
		log.Fatal("Migration failed:", err)
	}
	log.Println("Migrations completed successfully")

	// 3. Инициализация слоев
	tokenManager, err := auth.NewTokenManager(jwtSecret)
	if err != nil {
		log.Fatalf("failed to init token manager: %v", err)
	}
	// --- Orders ---
	orderRepo := orders.NewOrderRepository(db)
	orderService := orders.NewOrderService(orderRepo)
	orderHandler := orders.NewOrderHandler(orderService)

	// --- Offers ---
	offerRepo := offers.NewOfferRepository(db)
	offerService := offers.NewOfferService(offerRepo)
	offerHandler := offers.NewOfferHandler(offerService)

	// --- Auth ---
	authRepo := auth.NewRepository(db)
	authService := auth.NewService(authRepo, tokenManager, 30*time.Minute, 14*24*time.Hour)
	authHandler := auth.NewHandler(authService)

	// File Storage
	minioEndpoint := "localhost:9000"
	minioAccessKey := os.Getenv("MINIO_ROOT_USER")
	minioSecretKey := os.Getenv("MINIO_ROOT_PASSWORD")
	minioBucket := "food-images"
	minioUseSSL := false

	fileStorage, err := filestorage.NewFileStorage(minioEndpoint, minioAccessKey, minioSecretKey, minioBucket, minioUseSSL)
	if err != nil {
		log.Fatalf("failed to init file storage: %v", err)
	}
	log.Printf("File storage initialized for endpoint: %s", minioEndpoint)
	_ = fileStorage // Will be used in future handlers

	// --- Restaurants ---
	restaurantsRepo := restaurants.NewRepository(db)
	restaurantsService := restaurants.NewService(restaurantsRepo, fileStorage)
	restaurantsHandler := restaurants.NewHandler(restaurantsService)

	// --- Categories ---
	categoriesRepo := categories.NewRepository(db)
	categoriesService := categories.NewService(categoriesRepo)
	categoriesHandler := categories.NewHandler(categoriesService)

	// 4. Роутер
	router := gin.Default()
	app.NewRouter(router, authHandler, restaurantsHandler, categoriesHandler, orderHandler, offerHandler, jwtSecret)

	log.Println("Server starting on :8080")
	if err := router.Run(":8080"); err != nil {
		log.Fatal("Server start failed:", err)
	}
}
