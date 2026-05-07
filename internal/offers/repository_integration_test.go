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

func strPtr(s string) *string {
	return &s
}

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
	user := os.Getenv("DB_USER")
	if user == "" {
		user = "postgres"
	}
	pass := os.Getenv("DB_PASSWORD")
	if pass == "" {
		pass = "hsefcsse243_secret_password_postgres"
	}
	dbname := os.Getenv("DB_NAME")
	if dbname == "" {
		dbname = "foodsharing_test_db"
	}

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		host, user, pass, dbname, port)

	db, err := postgres.NewDB(dsn)
	require.NoError(t, err)

	// КРИТИЧЕСКИЙ ФИКС: Блокировка, чтобы тесты разных пакетов не дрались за базу
	db.Exec("SELECT pg_advisory_lock(123456)")

	cleanupSQL := `
		DROP SCHEMA IF EXISTS public CASCADE; 
		CREATE SCHEMA public; 
		GRANT ALL ON SCHEMA public TO postgres; 
		GRANT ALL ON SCHEMA public TO public;
		SET search_path TO public;
	`
	require.NoError(t, db.Exec(cleanupSQL).Error)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, goose.Up(sqlDB, "../../migrations"))

	// Разблокируем только ПОСЛЕ того, как накатили миграции
	db.Exec("SELECT pg_advisory_unlock(123456)")

	return db
}

func TestOfferRepository_Integration(t *testing.T) {
	db := setupOffersTestDB(t)
	repo := NewOfferRepository(db)
	ctx := context.Background()

	partner := &domain.User{ID: uuid.New(), Email: uuid.NewString() + "@test.com", Role: "PARTNER"}
	require.NoError(t, db.Create(partner).Error)

	category := &domain.Category{ID: uuid.New(), Name: "Тест Категория " + uuid.NewString()}
	require.NoError(t, db.Create(category).Error)

	restaurant := &domain.Restaurant{
		ID:        uuid.New(),
		PartnerID: partner.ID,
		Name:      "Пекарня",
		Address:   "ул. Мира",
		Latitude:  55.75,
		Longitude: 37.61,
	}

	require.NoError(t, db.Exec(`
		INSERT INTO restaurants (id, partner_id, name, address, latitude, longitude, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, NOW(), NOW())`,
		restaurant.ID, restaurant.PartnerID, restaurant.Name, restaurant.Address,
		restaurant.Latitude, restaurant.Longitude).Error)

	t.Run("Create and Get", func(t *testing.T) {
		offer := &domain.Offer{
			ID:                uuid.New(),
			RestaurantID:      restaurant.ID,
			CategoryID:        category.ID,
			Title:             "Круассан",
			Price:             100,
			OriginalPrice:     300,
			QuantityTotal:     5,
			QuantityAvailable: 5,
			IsActive:          true,
			PickupStart:       time.Now().Add(time.Hour),
			PickupEnd:         time.Now().Add(2 * time.Hour),
		}
		require.NoError(t, repo.Create(ctx, offer))

		saved, err := repo.GetByID(ctx, offer.ID)
		assert.NoError(t, err)
		assert.Equal(t, "Круассан", saved.Title)
	})

	t.Run("GetPartnerOffers handles virtual distance correctly", func(t *testing.T) {
		offers, total, err := repo.GetPartnerOffers(ctx, restaurant.ID, 10, 0)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, total, int64(1))
		assert.Nil(t, offers[0].Distance)
	})

	t.Run("GetPublicOffers with GeoLocation", func(t *testing.T) {
		// Смещаем координаты запроса на ~1км, чтобы дистанция НЕ была нулевой
		lat := 55.76
		lng := 37.62
		params := FilterParams{
			Lat: &lat, Lng: &lng, Limit: 10, Offset: 0,
		}

		offers, total, err := repo.GetPublicOffers(ctx, params)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, total, int64(1))
		require.NotEmpty(t, offers)
		assert.NotNil(t, offers[0].Distance)
		assert.Greater(t, *offers[0].Distance, 0) // Теперь тут будет > 0
	})
}
