package main

import (
	"context"
	"kursach_backend/internal/adminui"
	"kursach_backend/internal/analytics"
	emaildelivery "kursach_backend/internal/email"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/pressly/goose/v3"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"kursach_backend/internal/notifications"
	"kursach_backend/internal/offers"
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
	var db *gorm.DB
	var err error

	log.Printf("Connecting to database at %s:%s...", os.Getenv("DB_HOST"), os.Getenv("DB_PORT"))
	for i := 0; i < 5; i++ {
		db, err = postgres.NewDB(dsn)
		if err == nil {
			break
		}
		log.Printf("Database not ready, retrying in 2 seconds... (%d/5)", i+1)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatal("Failed to connect to database after retries:", err)
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET is required")
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("Failed to get sql.DB:", err)
	}

	log.Println("Running migrations via Goose...")
	err = goose.SetDialect("postgres")
	if err != nil {
		return
	}

	if err := goose.Up(sqlDB, "migrations"); err != nil {
		log.Fatal("Migration failed:", err)
	}

	log.Println("Migrations completed successfully")

	// 3. Инициализация слоев
	tokenManager, err := auth.NewTokenManager(jwtSecret)
	if err != nil {
		log.Fatalf("failed to init token manager: %v", err)
	}

	accessTTL, err := durationFromEnv("ACCESS_TOKEN_TTL", 30*time.Minute)
	if err != nil {
		log.Fatalf("invalid ACCESS_TOKEN_TTL: %v", err)
	}
	refreshTTL, err := durationFromEnv("REFRESH_TOKEN_TTL", 14*24*time.Hour)
	if err != nil {
		log.Fatalf("invalid REFRESH_TOKEN_TTL: %v", err)
	}

	// --- Orders ---
	orderRepo := orders.NewOrderRepository(db)
	notificationRepo := notifications.NewRepository(db)
	pushProvider := notifications.NewPushProviderFromEnv(context.Background())
	notificationService := notifications.NewService(notificationRepo, pushProvider)
	orderService := orders.NewOrderService(orderRepo, notificationService)
	orderHandler := orders.NewOrderHandler(orderService)

	// --- Offers ---
	offerRepo := offers.NewOfferRepository(db)
	offerService := offers.NewOfferService(offerRepo)
	offerHandler := offers.NewOfferHandler(offerService)

	// --- Auth ---
	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	emailSender, err := emaildelivery.NewSenderFromEnv()
	if err != nil {
		log.Fatalf("failed to init email sender: %v", err)
	}
	authRepo := auth.NewRepository(db)
	authService := auth.NewService(authRepo, tokenManager, accessTTL, refreshTTL, googleClientID, emailSender)
	authHandler := auth.NewHandler(authService)

	// File Storage
	minioEndpoint := os.Getenv("MINIO_ENDPOINT")
	if minioEndpoint == "" {
		minioEndpoint = "minio:9000"
	}
	minioAccessKey := os.Getenv("MINIO_ACCESS_KEY")
	if minioAccessKey == "" {
		minioAccessKey = os.Getenv("MINIO_ROOT_USER")
	}
	minioSecretKey := os.Getenv("MINIO_SECRET_KEY")
	if minioSecretKey == "" {
		minioSecretKey = os.Getenv("MINIO_ROOT_PASSWORD")
	}
	minioBucket := os.Getenv("MINIO_BUCKET")
	if minioBucket == "" {
		minioBucket = "food-images"
	}
	minioUseSSL, err := strconv.ParseBool(os.Getenv("MINIO_USE_SSL"))
	if err != nil {
		minioUseSSL = false
	}
	minioPublicBaseURL := os.Getenv("MINIO_PUBLIC_BASE_URL")

	fileStorage, err := filestorage.NewFileStorage(minioEndpoint, minioAccessKey, minioSecretKey, minioBucket, minioUseSSL, minioPublicBaseURL)
	if err != nil {
		log.Fatalf("Failed to init file storage: %v", err)
	} else {
		log.Printf("File storage initialized for endpoint: %s", minioEndpoint)
	}
	_ = fileStorage // Will be used in future handlers

	// --- Restaurants ---
	restaurantsRepo := restaurants.NewRepository(db)
	restaurantsService := restaurants.NewService(restaurantsRepo, fileStorage)
	restaurantsHandler := restaurants.NewHandler(restaurantsService)

	// --- Categories ---
	categoriesRepo := categories.NewRepository(db)
	categoriesService := categories.NewService(categoriesRepo)
	categoriesHandler := categories.NewHandler(categoriesService)
	adminHandler := adminui.NewHandler(authService, categoriesService, restaurantsService, jwtSecret)

	// --- Analytics ---
	analyticsRepo := analytics.NewAnalyticsRepository(db)
	analyticsService := analytics.NewAnalyticsService(analyticsRepo)
	analyticsHandler := analytics.NewAnalyticsHandler(analyticsService)

	notificationsHandler := notifications.NewNotificationsHandler(notificationService)

	// Cron-worker
	go analyticsService.StartAnalyticsWorker(context.Background())
	go orderService.StartExpirationWorker(context.Background())
	go orderService.StartNotificationWorker(context.Background())
	go notificationService.Start(context.Background())

	// 4. Роутер
	router := gin.Default()
	router.HandleMethodNotAllowed = true
	app.NewRouter(router, authService, authHandler, restaurantsHandler, categoriesHandler, orderHandler, offerHandler, analyticsHandler, notificationsHandler, adminHandler, jwtSecret)

	appPort := os.Getenv("APP_PORT")
	if appPort == "" {
		appPort = "8080"
	}

	log.Printf("Server starting on :%s", appPort)
	if err := router.Run(":" + appPort); err != nil {
		log.Fatal("Server start failed:", err)
	}
}

func durationFromEnv(key string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}
