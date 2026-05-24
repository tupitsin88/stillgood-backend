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

func TestOfferRepositoryGetPublicOffersSearch(t *testing.T) {
	db := setupOffersIntegrationDB(t)
	repo := NewOfferRepository(db)
	ctx := context.Background()
	now := time.Now()

	partner := &domain.User{ID: uuid.New(), Email: uuid.NewString() + "@t.com", Role: "PARTNER", Name: "Partner"}
	require.NoError(t, db.Create(partner).Error)

	rollsCategory := &domain.Category{ID: uuid.New(), Name: "Rolls"}
	dessertCategory := &domain.Category{ID: uuid.New(), Name: "Desserts"}
	require.NoError(t, db.Create(&[]domain.Category{*rollsCategory, *dessertCategory}).Error)

	sakura := domain.Restaurant{
		ID:        uuid.New(),
		PartnerID: partner.ID,
		Name:      "Sakura Box",
		Address:   "Sakura street",
		Latitude:  55.75,
		Longitude: 37.61,
		IsActive:  true,
	}
	green := domain.Restaurant{
		ID:        uuid.New(),
		PartnerID: partner.ID,
		Name:      "Green Cafe",
		Address:   "Green street",
		Latitude:  55.76,
		Longitude: 37.62,
		IsActive:  true,
	}
	require.NoError(t, db.Omit("Location").Create(&[]domain.Restaurant{sakura, green}).Error)

	offers := []domain.Offer{
		{
			ID:                uuid.New(),
			RestaurantID:      sakura.ID,
			CategoryID:        rollsCategory.ID,
			Title:             "Сет Филадельфия",
			Price:             300,
			OriginalPrice:     600,
			QuantityAvailable: 3,
			QuantityTotal:     3,
			PickupStart:       now.Add(time.Hour),
			PickupEnd:         now.Add(2 * time.Hour),
			IsActive:          true,
		},
		{
			ID:                uuid.New(),
			RestaurantID:      sakura.ID,
			CategoryID:        dessertCategory.ID,
			Title:             "Чизкейк",
			Price:             150,
			OriginalPrice:     300,
			QuantityAvailable: 2,
			QuantityTotal:     2,
			PickupStart:       now.Add(2 * time.Hour),
			PickupEnd:         now.Add(3 * time.Hour),
			IsActive:          true,
		},
		{
			ID:                uuid.New(),
			RestaurantID:      sakura.ID,
			CategoryID:        rollsCategory.ID,
			Title:             "Ланч бокс",
			Price:             250,
			OriginalPrice:     500,
			QuantityAvailable: 4,
			QuantityTotal:     4,
			PickupStart:       now.Add(3 * time.Hour),
			PickupEnd:         now.Add(4 * time.Hour),
			IsActive:          true,
		},
		{
			ID:                uuid.New(),
			RestaurantID:      green.ID,
			CategoryID:        rollsCategory.ID,
			Title:             "Роллы с тунцом",
			Price:             200,
			OriginalPrice:     400,
			QuantityAvailable: 1,
			QuantityTotal:     1,
			PickupStart:       now.Add(4 * time.Hour),
			PickupEnd:         now.Add(5 * time.Hour),
			IsActive:          true,
		},
	}
	require.NoError(t, db.Create(&offers).Error)

	t.Run("q finds offer by title", func(t *testing.T) {
		got, total, err := repo.GetPublicOffers(ctx, FilterParams{
			Query:  "  роллы  ",
			Limit:  20,
			Offset: 0,
		})

		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, int64(1), total)
		assert.Equal(t, "Роллы с тунцом", got[0].Title)
	})

	t.Run("blank q does not filter", func(t *testing.T) {
		got, total, err := repo.GetPublicOffers(ctx, FilterParams{
			Query:  "   ",
			Limit:  20,
			Offset: 0,
		})

		require.NoError(t, err)
		require.Len(t, got, 4)
		assert.Equal(t, int64(4), total)
	})

	t.Run("q finds offers by restaurant name", func(t *testing.T) {
		got, total, err := repo.GetPublicOffers(ctx, FilterParams{
			Query:  "sAkUrA",
			Limit:  20,
			Offset: 0,
		})

		require.NoError(t, err)
		require.Len(t, got, 3)
		assert.Equal(t, int64(3), total)
		for _, offer := range got {
			assert.Equal(t, "Sakura Box", offer.Restaurant.Name)
		}
	})

	t.Run("q works with categoryId", func(t *testing.T) {
		got, total, err := repo.GetPublicOffers(ctx, FilterParams{
			Query:      "Sakura",
			CategoryID: &dessertCategory.ID,
			Limit:      20,
			Offset:     0,
		})

		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, int64(1), total)
		assert.Equal(t, "Чизкейк", got[0].Title)
	})

	t.Run("pagination total is counted after search", func(t *testing.T) {
		got, total, err := repo.GetPublicOffers(ctx, FilterParams{
			Query:  "Sakura",
			Limit:  1,
			Offset: 0,
		})

		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, int64(3), total)
	})
}
