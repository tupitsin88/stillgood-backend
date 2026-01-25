package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"kursach_backend/internal/domain"
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

	// 2. Авто-миграции (создадут таблицы по структурам)
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

	// 3. Инициализация слоев (Orders)
	orderRepo := orders.NewOrderRepository(db)
	orderService := orders.NewOrderService(orderRepo)
	orderHandler := orders.NewOrderHandler(orderService)

	// 4. Роутер
	r := gin.Default()

	// Простейший middleware для user_id (заглушка для теста)
	// В реальности тут будет JWT middleware
	r.Use(func(c *gin.Context) {
		// ID юзера = 1 для тестов
		c.Set("user_id", 1)
		c.Next()
	})

	v1 := r.Group("/api/v1")
	{
		ordersGroup := v1.Group("/orders")
		{
			ordersGroup.POST("", orderHandler.CreateOrder)
			ordersGroup.POST("/:id/pay", orderHandler.PayOrder)
			ordersGroup.POST("/:id/cancel", orderHandler.CancelOrder)
			ordersGroup.GET("", orderHandler.GetUserOrders)
		}

		partnerGroup := v1.Group("/partner/orders")
		{
			partnerGroup.GET("", orderHandler.GetPartnerOrders)
			partnerGroup.POST("/:id/complete", func(c *gin.Context) {
				orderHandler.CompleteOrder(c)
			})
		}
	}

	// 5. Запуск
	log.Println("Server starting on :8080")
	r.Run(":8080")
}
