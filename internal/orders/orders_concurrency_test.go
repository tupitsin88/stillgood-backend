package orders

import (
	"context"
	"fmt"
	"kursach_backend/internal/domain"
	"kursach_backend/pkg/postgres"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func setupTestDB() *gorm.DB {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_PORT"),
	)
	db, err := postgres.NewDB(dsn)
	if err != nil {
		panic(fmt.Sprintf("DB connection failed: %v", err))
	}
	return db
}

func TestCreateOrder_Concurrency(t *testing.T) {
	db := setupTestDB()
	repo := NewOrderRepository(db)
	service := NewOrderService(repo, &LogNotificationProvider{})
	ctx := context.Background()

	partner := &domain.User{ID: uuid.New(), Email: "test@partner.com", Role: "PARTNER"}
	db.FirstOrCreate(partner, "email = ?", partner.Email)

	category := &domain.Category{ID: uuid.New(), Name: "Тестовая категория"}
	db.FirstOrCreate(category, "name = ?", category.Name)

	restaurant := &domain.Restaurant{
		ID:        uuid.New(),
		Name:      "Test Food",
		PartnerID: partner.ID,
		Address:   "Test Street",
	}
	db.FirstOrCreate(restaurant, "name = ?", restaurant.Name)

	offerID := uuid.New()
	testOffer := &domain.Offer{
		ID:                offerID,
		RestaurantID:      restaurant.ID,
		CategoryID:        category.ID,
		Title:             "Last dance",
		Price:             100,
		OriginalPrice:     1000,
		QuantityAvailable: 1,
		QuantityTotal:     1,
		IsActive:          true,
		PickupStart:       time.Now(),
		PickupEnd:         time.Now().Add(time.Hour),
	}
	err := db.Create(testOffer).Error
	assert.NoError(t, err)
	const numReqs = 10
	userIDs := make([]uuid.UUID, numReqs)
	for i := 0; i < numReqs; i++ {
		userIDs[i] = uuid.New()
		user := &domain.User{
			ID:    userIDs[i],
			Email: fmt.Sprintf("user_%d@test.com", i),
			Name:  fmt.Sprintf("User %d", i),
			Role:  "USER",
		}
		db.Create(user)
	}
	var wg sync.WaitGroup
	results := make(chan error, numReqs)
	wg.Add(numReqs)
	for i := 0; i < numReqs; i++ {
		currentUserID := userIDs[i]
		go func(uID uuid.UUID) {
			defer wg.Done()
			_, err := service.CreateOrder(ctx, uID, CreateOrderRequest{
				OfferID: offerID.String(),
			})
			results <- err
		}(currentUserID)
	}
	wg.Wait()
	close(results)
	successCount := 0
	failCount := 0
	for err := range results {
		if err != nil {
			failCount++
		} else {
			successCount++
		}
	}
	assert.Equal(t, 1, successCount, "Должен быть ровно один успешный заказ")
	assert.Equal(t, 9, failCount, "9 человек должны были получить ошибку отсутствия товара")

	var updatedOffer domain.Offer
	db.First(&updatedOffer, "id = ?", offerID)
	assert.Equal(t, 0, updatedOffer.QuantityAvailable, "Остаток боксов должен стать 0, а не отрицательным")
}
