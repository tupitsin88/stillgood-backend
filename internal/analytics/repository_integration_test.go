package analytics

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"kursach_backend/internal/domain"
	"kursach_backend/pkg/postgres"

	"github.com/google/uuid"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func strPtr(s string) *string {
	return &s
}

func setupAnalyticsIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("DB_PORT")
	if port == "" {
		if os.Getenv("GITHUB_ACTIONS") == "true" {
			port = "5432"
		} else {
			port = "5433"
		}
	}
	dsn := fmt.Sprintf("host=%s user=postgres password=hsefcsse243_secret_password_postgres dbname=foodsharing_test_db port=%s sslmode=disable", host, port)

	db, err := postgres.NewDB(dsn)
	require.NoError(t, err, "Не удалось подключиться к тестовой БД")
	db.Exec("SELECT pg_advisory_lock(123456)")
	t.Cleanup(func() {
		db.Exec("SELECT pg_advisory_unlock(123456)")
	})
	sqlDB, _ := db.DB()

	require.NoError(t, goose.Up(sqlDB, "../../migrations"))

	err = db.Exec("TRUNCATE users, categories, restaurants, offers, orders, order_status_histories, daily_analytics RESTART IDENTITY CASCADE").Error
	require.NoError(t, err)

	return db
}

func TestAnalyticsRepository_Integration(t *testing.T) {
	db := setupAnalyticsIntegrationDB(t)
	repo := NewAnalyticsRepository(db)
	ctx := context.Background()
	testDate := time.Now().UTC().Truncate(24 * time.Hour)
	partner := &domain.User{ID: uuid.New(), Email: "p@test.com", Role: "PARTNER", Name: "Partner"}
	require.NoError(t, db.Create(partner).Error)
	customer := &domain.User{ID: uuid.New(), Email: "c@test.com", Role: "USER", Name: "Customer"}
	require.NoError(t, db.Create(customer).Error)
	category := &domain.Category{ID: uuid.New(), Name: "Бургеры"}
	require.NoError(t, db.Create(category).Error)
	restaurant := &domain.Restaurant{
		ID:         uuid.New(),
		PartnerID:  partner.ID,
		Name:       "Burger Queen",
		Commission: 15.0,
	}
	require.NoError(t, db.Omit("Location").Create(restaurant).Error)

	offer := &domain.Offer{
		ID:            uuid.New(),
		RestaurantID:  restaurant.ID,
		CategoryID:    category.ID,
		Title:         "Combo Box",
		Price:         1000,
		OriginalPrice: 2000,
	}
	require.NoError(t, db.Create(offer).Error)
	orderA := &domain.Order{ID: uuid.New(), OfferID: offer.ID, UserID: customer.ID, Amount: 1000, Status: domain.OrderCompleted, CreatedAt: testDate}
	require.NoError(t, db.Create(orderA).Error)
	require.NoError(t, db.Create(&domain.OrderStatusHistory{ID: uuid.New(), OrderID: orderA.ID, Status: domain.OrderCompleted, ChangedAt: testDate}).Error)
	orderB := &domain.Order{
		ID:                 uuid.New(),
		OfferID:            offer.ID,
		UserID:             customer.ID,
		Amount:             1000,
		Status:             domain.OrderCancelled,
		CancellationReason: strPtr("expired"),
		CreatedAt:          testDate,
	}
	require.NoError(t, db.Create(orderB).Error)
	require.NoError(t, db.Create(&domain.OrderStatusHistory{ID: uuid.New(), OrderID: orderB.ID, Status: domain.OrderCancelled, ChangedAt: testDate}).Error)

	t.Run("AggregateDailyStats correctly calculates financial metrics", func(t *testing.T) {
		stats, err := repo.AggregateDailyStats(ctx, testDate)

		require.NoError(t, err)
		require.Len(t, stats, 1)

		s := stats[0]
		assert.Equal(t, restaurant.ID, s.RestaurantID)
		assert.Equal(t, 2, s.TotalBookings)
		assert.Equal(t, 1, s.CompletedOrders)
		assert.Equal(t, 1, s.CancelledOrders)
		assert.Equal(t, 1, s.ExpiredOrders)

		assert.Equal(t, 1000.0, s.GrossRevenue)
		assert.Equal(t, 150.0, s.ServiceFee)
		assert.Equal(t, 850.0, s.NetPayout)
	})

	t.Run("SaveStats handles Upsert conflict", func(t *testing.T) {
		initial := []domain.DailyAnalytics{{
			RestaurantID:  restaurant.ID,
			Date:          testDate,
			CategoryName:  "Бургеры",
			TotalBookings: 5,
		}}
		err := repo.SaveStats(ctx, initial)
		assert.NoError(t, err)
		updated := initial
		updated[0].TotalBookings = 99
		err = repo.SaveStats(ctx, updated)
		assert.NoError(t, err)
		var check domain.DailyAnalytics
		db.First(&check, "restaurant_id = ?", restaurant.ID)
		assert.Equal(t, 99, check.TotalBookings, "Данные должны были обновиться (Upsert)")
	})
}
