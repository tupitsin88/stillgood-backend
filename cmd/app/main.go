package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"kursach_backend/internal/domain"
	"kursach_backend/internal/offers"
	"kursach_backend/internal/orders"
)

func main() {
	// 1. Подключение к БД
	dsn := "host=" + os.Getenv("DB_HOST") +
		" user=" + os.Getenv("DB_USER") +
		" password=" + os.Getenv("DB_PASSWORD") +
		" dbname=" + os.Getenv("DB_NAME") +
		" port=" + os.Getenv("DB_PORT") +
		" sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// 2. Авто-миграции
	log.Println("Running migrations...")
	err = db.AutoMigrate(
		&domain.User{},
		&domain.Restaurant{},
		&domain.Offer{},
		&domain.Order{},
		&domain.OrderStatusHistory{},
	)
	if err != nil {
		log.Fatal("Migration failed:", err)
	}
	log.Println("Migrations completed successfully")

	// 3. Инициализация слоев
	// --- Orders ---
	orderRepo := orders.NewOrderRepository(db)
	orderService := orders.NewOrderService(orderRepo)
	orderHandler := orders.NewOrderHandler(orderService)

	// --- Offers ---
	offerRepo := offers.NewOfferRepository(db)
	offerService := offers.NewOfferService(offerRepo)
	offerHandler := offers.NewOfferHandler(offerService)
	// 4. Роутер
	r := gin.Default()

	// Mock Middleware (ВРЕМЕННАЯ ЗАГЛУШКА)
	// В реальном Auth сервисе тут будет валидация JWT
	mockAuthMiddleware := func(c *gin.Context) {
		// Хардкодим ID юзера = 1 для тестов
		c.Set("user_id", "00000000-0000-0000-0000-000000000001")
		c.Next()
	}

	orders.RegisterRoutes(r, orderHandler, mockAuthMiddleware)
	offers.RegisterRoutes(r, offerHandler, mockAuthMiddleware)

	log.Println("Server starting on :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatal("Server start failed:", err)
	}
}
