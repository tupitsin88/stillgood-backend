package main

import (
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"kursach_backend/internal/auth"
	"kursach_backend/internal/domain"
)

func main() {
	dsn := "host=localhost user=postgres password=password dbname=foodsharing_db port=5433 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	db.AutoMigrate(&domain.User{})

	authRepo := auth.NewRepository(db)
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "supersecretkey"
	}
	tokenManager, err := auth.NewTokenManager(jwtSecret)
	if err != nil {
		panic(err)
	}

	authService := auth.NewService(authRepo, tokenManager, 30*time.Minute, 14*24*time.Hour)
	authHandler := auth.NewHandler(authService)
	router := gin.Default()
	api := router.Group("/api/v1")

	authHandler.InitRoutes(api)

	router.Run(":8080")
}
