package offers

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

func setupOffersTestDB(t *testing.T) *gorm.DB {
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

func TestOfferRepository_Integration(t *testing.T) {
	db := setupOffersTestDB(t)
	repo := NewOfferRepository(db)
	ctx := context.Background()

	partner := &domain.User{
		ID:    uuid.New(),
		Email: "partner@test.com",
		Role:  "PARTNER",
	}
	require.NoError(t, db.Create(partner).Error)

	category := &domain.Category{
		ID:   uuid.New(),
		Name: "Выпечка",
	}
	require.NoError(t, db.Create(category).Error)

	restaurant := &domain.Restaurant{
		ID:        uuid.New(),
		PartnerID: partner.ID,
		Name:      "Тестовая Пекарня",
		Address:   "ул. Тестовая, 1",
		Latitude:  55.7558,
		Longitude: 37.6173,
	}

	require.NoError(t, db.Exec(`
		INSERT INTO restaurants (id, partner_id, name, address, latitude, longitude, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, NOW(), NOW())`,
		restaurant.ID, restaurant.PartnerID, restaurant.Name, restaurant.Address,
		restaurant.Latitude, restaurant.Longitude).Error)

	t.Run("Create and GetByID", func(t *testing.T) {
		offer := &domain.Offer{
			ID:                uuid.New(),
			RestaurantID:      restaurant.ID,
			CategoryID:        category.ID,
			Title:             "Бокс с круассанами",
			Price:             300,
			OriginalPrice:     900,
			QuantityTotal:     10,
			QuantityAvailable: 10,
			IsActive:          true,
			ImageURL:          strPtr("http://cdn.com/img.jpg"),
			PickupStart:       time.Now().Add(time.Hour),
			PickupEnd:         time.Now().Add(3 * time.Hour),
		}

		err := repo.Create(ctx, offer)
		assert.NoError(t, err)
		saved, err := repo.GetByID(ctx, offer.ID)
		require.NoError(t, err)
		assert.Equal(t, "Бокс с круассанами", saved.Title)
		assert.NotNil(t, saved.Restaurant)
		assert.NotNil(t, saved.Category)
	})

	t.Run("Update", func(t *testing.T) {
		var offer domain.Offer
		db.First(&offer)

		offer.Title = "Обновленный бокс"
		offer.QuantityAvailable = 5

		err := repo.Update(ctx, &offer)
		assert.NoError(t, err)

		var updated domain.Offer
		db.First(&updated, offer.ID)
		assert.Equal(t, "Обновленный бокс", updated.Title)
		assert.Equal(t, 5, updated.QuantityAvailable)
	})

	t.Run("GetPartnerOffers handles virtual distance correctly", func(t *testing.T) {
		offers, total, err := repo.GetPartnerOffers(ctx, restaurant.ID, 10, 0)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, total, int64(1))
		assert.NotEmpty(t, offers)
		assert.Nil(t, offers[0].Distance)
	})

	t.Run("GetPublicOffers with GeoLocation", func(t *testing.T) {
		lat := 55.75
		lng := 37.61
		params := FilterParams{
			Lat:    &lat,
			Lng:    &lng,
			Limit:  10,
			Offset: 0,
		}

		offers, total, err := repo.GetPublicOffers(ctx, params)

		assert.NoError(t, err)
		assert.GreaterOrEqual(t, total, int64(1))
		require.NotEmpty(t, offers)
		assert.NotNil(t, offers[0].Distance)
		assert.Greater(t, *offers[0].Distance, 0)
	})

	t.Run("Delete", func(t *testing.T) {
		var offer domain.Offer
		db.First(&offer)

		err := repo.Delete(ctx, offer.ID)
		assert.NoError(t, err)

		var check domain.Offer
		err = db.First(&check, "id = ?", offer.ID).Error
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})
}
