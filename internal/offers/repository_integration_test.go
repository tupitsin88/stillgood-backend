package offers

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

func setupOffersTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "5433"
	}
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		port = "5432"
	}

	dsn := fmt.Sprintf("host=%s user=postgres password=hsefcsse243_secret_password_postgres dbname=foodsharing_test_db port=%s sslmode=disable", host, port)

	db, err := postgres.NewDB(dsn)
	require.NoError(t, err)
	db.Exec("SELECT pg_advisory_lock(123456)")
	defer db.Exec("SELECT pg_advisory_unlock(123456)")

	sqlDB, _ := db.DB()
	require.NoError(t, goose.Up(sqlDB, "../../migrations"))
	require.NoError(t, db.Exec("TRUNCATE offers, restaurants, categories, users, order_status_histories, orders RESTART IDENTITY CASCADE").Error)

	return db
}

func TestOfferRepository_Integration(t *testing.T) {
	db := setupOffersTestDB(t)
	repo := NewOfferRepository(db)
	ctx := context.Background()
	testID := uuid.NewString()
	partner := &domain.User{ID: uuid.New(), Email: "p-" + testID + "@t.com", Role: "PARTNER", Name: "Partner"}
	require.NoError(t, db.Create(partner).Error)

	category := &domain.Category{ID: uuid.New(), Name: "Cat-" + testID}
	require.NoError(t, db.Create(category).Error)

	restaurant := &domain.Restaurant{
		ID:        uuid.New(),
		PartnerID: partner.ID,
		Name:      "Rest-" + testID,
		Address:   "Street",
		Latitude:  55.75,
		Longitude: 37.61,
	}
	require.NoError(t, db.Omit("Location").Create(restaurant).Error)

	t.Run("Create and Get", func(t *testing.T) {
		offer := &domain.Offer{
			ID:                uuid.New(),
			RestaurantID:      restaurant.ID,
			CategoryID:        category.ID,
			Title:             "Test Offer",
			Price:             100,
			OriginalPrice:     200,
			QuantityTotal:     5,
			QuantityAvailable: 5,
			IsActive:          true,
			PickupStart:       time.Now().Add(time.Hour),
			PickupEnd:         time.Now().Add(2 * time.Hour),
		}
		require.NoError(t, repo.Create(ctx, offer))

		saved, err := repo.GetByID(ctx, offer.ID)
		assert.NoError(t, err)
		assert.Equal(t, "Test Offer", saved.Title)
	})

	t.Run("GetPublicOffers with Geo", func(t *testing.T) {
		lat, lng := 55.76, 37.62
		params := FilterParams{Lat: &lat, Lng: &lng, Limit: 10}
		offers, _, err := repo.GetPublicOffers(ctx, params)
		assert.NoError(t, err)
		if len(offers) > 0 {
			assert.NotNil(t, offers[0].Distance)
			assert.Greater(t, *offers[0].Distance, 0)
		}
	})
}
