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

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

type testDBConfig struct {
	host     string
	user     string
	password string
	name     string
	port     string
}

func (c testDBConfig) dsn() string {
	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		c.host, c.user, c.password, c.name, c.port)
}

func setupOffersIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()

	cfg := testDBConfig{
		host:     envOrDefault("TEST_DB_HOST", envOrDefault("DB_HOST", "localhost")),
		user:     envOrDefault("TEST_DB_USER", envOrDefault("DB_USER", "postgres")),
		password: envOrDefault("TEST_DB_PASSWORD", envOrDefault("DB_PASSWORD", "hsefcsse243_secret_password_postgres")),
		name:     envOrDefault("TEST_DB_NAME", "foodsharing_test_db"),
		port:     envOrDefault("TEST_DB_PORT", envOrDefault("DB_PORT", "5433")),
	}

	db, err := postgres.NewDB(cfg.dsn())
	require.NoError(t, err)

	db.Exec("SELECT pg_advisory_lock(123456)")
	t.Cleanup(func() {
		db.Exec("SELECT pg_advisory_unlock(123456)")
	})

	require.NoError(t, db.Exec("DROP SCHEMA IF EXISTS public CASCADE; CREATE SCHEMA public; SET search_path TO public;").Error)
	sqlDB, _ := db.DB()
	require.NoError(t, goose.Up(sqlDB, "../../migrations"))

	return db
}

func TestOfferRepository_Integration(t *testing.T) {
	db := setupOffersIntegrationDB(t)
	repo := NewOfferRepository(db)
	ctx := context.Background()

	partner := &domain.User{ID: uuid.New(), Email: uuid.NewString() + "@t.com", Role: "PARTNER", Name: "Partner"}
	require.NoError(t, db.Create(partner).Error)

	category := &domain.Category{ID: uuid.New(), Name: "Cat " + uuid.NewString()}
	require.NoError(t, db.Create(category).Error)

	restaurant := &domain.Restaurant{ID: uuid.New(), PartnerID: partner.ID, Name: "Rest", Latitude: 55.75, Longitude: 37.61}
	require.NoError(t, db.Omit("Location").Create(restaurant).Error)

	t.Run("Create and Get", func(t *testing.T) {
		offer := &domain.Offer{
			ID: uuid.New(), RestaurantID: restaurant.ID, CategoryID: category.ID,
			Title: "Croissant", Price: 100, OriginalPrice: 300, QuantityTotal: 5, QuantityAvailable: 5,
			PickupStart: time.Now().Add(time.Hour), PickupEnd: time.Now().Add(2 * time.Hour),
		}
		require.NoError(t, repo.Create(ctx, offer))

		saved, err := repo.GetByID(ctx, offer.ID)
		assert.NoError(t, err)
		assert.Equal(t, "Croissant", saved.Title)
	})
}
