package main

import (
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"kursach_backend/internal/app"
	"kursach_backend/internal/auth"
	"kursach_backend/internal/domain"
	"kursach_backend/pkg/postgres"
)

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
	if err := db.AutoMigrate(&domain.User{}); err != nil {
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

	// 4. Router
	router := gin.Default()
	app.NewRouter(router, authHandler, jwtSecret)

	// 5. Run
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
