package analytics

import (
	"context"
	"fmt"
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

func setupAnalyticsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		"localhost", "postgres", "hsefcsse243_secret_password_postgres", "foodsharing_test_db", "5433")

	db, err := postgres.NewDB(dsn)
	require.NoError(t, err)
	require.NoError(t, db.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public;").Error)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, goose.Up(sqlDB, "../../migrations"))
	return db
}

func TestAnalyticsRepository_AggregateDailyStats(t *testing.T) {
	db := setupAnalyticsTestDB(t)
	repo := NewAnalyticsRepository(db)
	ctx := context.Background()
	testDate := time.Now().UTC().Truncate(24 * time.Hour)

	partner := &domain.User{ID: uuid.New(), Email: "analytics-p@test.com", Role: "PARTNER"}
	require.NoError(t, db.Create(partner).Error)

	customer := &domain.User{ID: uuid.New(), Email: "customer@test.com", Role: "USER"}
	require.NoError(t, db.Create(customer).Error)

	category := &domain.Category{ID: uuid.New(), Name: "Тест Еда"}
	require.NoError(t, db.Create(category).Error)

	restaurant := &domain.Restaurant{
		ID:         uuid.New(),
		PartnerID:  partner.ID,
		Name:       "Analytics Rest",
		Commission: 10.0,
		IsActive:   true,
	}
	require.NoError(t, db.Create(restaurant).Error)

	offer := &domain.Offer{
		ID:           uuid.New(),
		RestaurantID: restaurant.ID,
		CategoryID:   category.ID,
		Title:        "Test Box",
		Price:        500,
	}
	require.NoError(t, db.Create(offer).Error)

	order1 := &domain.Order{ID: uuid.New(), OfferID: offer.ID, UserID: customer.ID, Amount: 500, Status: domain.OrderCompleted, CreatedAt: testDate}
	require.NoError(t, db.Create(order1).Error)
	require.NoError(t, db.Create(&domain.OrderStatusHistory{ID: uuid.New(), OrderID: order1.ID, Status: domain.OrderCompleted, ChangedAt: testDate}).Error)

	order2 := &domain.Order{ID: uuid.New(), OfferID: offer.ID, UserID: customer.ID, Amount: 500, Status: domain.OrderCancelled, CreatedAt: testDate}
	require.NoError(t, db.Create(order2).Error)
	require.NoError(t, db.Create(&domain.OrderStatusHistory{ID: uuid.New(), OrderID: order2.ID, Status: domain.OrderCancelled, ChangedAt: testDate}).Error)

	order3 := &domain.Order{
		ID:                 uuid.New(),
		OfferID:            offer.ID,
		UserID:             customer.ID,
		Amount:             500,
		Status:             domain.OrderCancelled,
		CancellationReason: strPtr("expired"),
		CreatedAt:          testDate,
	}
	require.NoError(t, db.Create(order3).Error)
	require.NoError(t, db.Create(&domain.OrderStatusHistory{ID: uuid.New(), OrderID: order3.ID, Status: domain.OrderCancelled, ChangedAt: testDate}).Error)
	stats, err := repo.AggregateDailyStats(ctx, testDate)
	require.NoError(t, err)
	require.Len(t, stats, 1)
	s := stats[0]
	assert.Equal(t, restaurant.ID, s.RestaurantID)
	assert.Equal(t, 3, s.TotalBookings)
	assert.Equal(t, 1, s.CompletedOrders)
	assert.Equal(t, 2, s.CancelledOrders)
	assert.Equal(t, 1, s.ExpiredOrders)
	assert.Equal(t, 500.0, s.GrossRevenue)
	assert.Equal(t, 50.0, s.ServiceFee)
	assert.Equal(t, 450.0, s.NetPayout)
}

func TestAnalyticsRepository_SaveStats_Upsert(t *testing.T) {
	db := setupAnalyticsTestDB(t)
	repo := NewAnalyticsRepository(db)
	ctx := context.Background()
	testDate := time.Now().UTC().Truncate(24 * time.Hour)

	partner := &domain.User{ID: uuid.New(), Email: "upsert-p@test.com", Role: "PARTNER"}
	require.NoError(t, db.Create(partner).Error)
	restaurant := &domain.Restaurant{ID: uuid.New(), PartnerID: partner.ID, Name: "Upsert Rest"}
	require.NoError(t, db.Create(restaurant).Error)

	initialStat := []domain.DailyAnalytics{{
		RestaurantID:  restaurant.ID,
		Date:          testDate,
		CategoryName:  "Pizza",
		TotalBookings: 10,
	}}

	t.Run("Saves new stats", func(t *testing.T) {
		err := repo.SaveStats(ctx, initialStat)
		assert.NoError(t, err)

		var saved domain.DailyAnalytics
		err = db.First(&saved, "restaurant_id = ?", restaurant.ID).Error
		require.NoError(t, err)
		assert.Equal(t, 10, saved.TotalBookings)
	})

	t.Run("Updates existing stats (Upsert)", func(t *testing.T) {
		updatedStat := []domain.DailyAnalytics{{
			RestaurantID:  restaurant.ID,
			Date:          testDate,
			CategoryName:  "Pizza",
			TotalBookings: 25,
		}}

		err := repo.SaveStats(ctx, updatedStat)
		assert.NoError(t, err)

		var saved domain.DailyAnalytics
		db.First(&saved, "restaurant_id = ?", restaurant.ID)
		assert.Equal(t, 25, saved.TotalBookings)
	})
}
