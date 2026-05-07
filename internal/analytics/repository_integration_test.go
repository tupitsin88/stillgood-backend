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

func setupAnalyticsIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()

	cfg := testDBConfig{
		host:     envOrDefault("TEST_DB_HOST", envOrDefault("DB_HOST", "localhost")),
		user:     envOrDefault("TEST_DB_USER", envOrDefault("DB_USER", "postgres")),
		password: envOrDefault("TEST_DB_PASSWORD", envOrDefault("DB_PASSWORD", "hsefcsse243_secret_password_postgres")),
		name:     envOrDefault("TEST_DB_NAME", "foodsharing_test_db"),
		port:     envOrDefault("TEST_DB_PORT", envOrDefault("DB_PORT", "5433")),
	}

	db, err := postgres.NewDB(cfg.dsn())
	require.NoError(t, err, "Не удалось подключиться к тестовой БД")
	db.Exec("SELECT pg_advisory_lock(123456)")
	t.Cleanup(func() {
		db.Exec("SELECT pg_advisory_unlock(123456)")
	})

	require.NoError(t, db.Exec("DROP SCHEMA IF EXISTS public CASCADE; CREATE SCHEMA public; SET search_path TO public;").Error)

	sqlDB, _ := db.DB()
	require.NoError(t, goose.Up(sqlDB, "../../migrations"))

	return db
}

func TestAnalyticsRepository_Integration(t *testing.T) {
	db := setupAnalyticsIntegrationDB(t)
	repo := NewAnalyticsRepository(db)
	ctx := context.Background()
	testDate := time.Now().UTC().Truncate(24 * time.Hour)

	partner := &domain.User{ID: uuid.New(), Email: uuid.NewString() + "@test.com", Role: "PARTNER", Name: "Partner"}
	require.NoError(t, db.Create(partner).Error)

	customer := &domain.User{ID: uuid.New(), Email: uuid.NewString() + "@test.com", Role: "USER", Name: "Customer"}
	require.NoError(t, db.Create(customer).Error)

	category := &domain.Category{ID: uuid.New(), Name: "Food " + uuid.NewString()}
	require.NoError(t, db.Create(category).Error)

	restaurant := &domain.Restaurant{ID: uuid.New(), PartnerID: partner.ID, Name: "Test Rest", Commission: 10.0}
	require.NoError(t, db.Omit("Location").Create(restaurant).Error)

	offer := &domain.Offer{ID: uuid.New(), RestaurantID: restaurant.ID, CategoryID: category.ID, Title: "Box", Price: 500}
	require.NoError(t, db.Create(offer).Error)

	t.Run("Aggregate Stats", func(t *testing.T) {
		order := &domain.Order{ID: uuid.New(), OfferID: offer.ID, UserID: customer.ID, Amount: 500, Status: domain.OrderCompleted, CreatedAt: testDate}
		require.NoError(t, db.Create(order).Error)
		require.NoError(t, db.Create(&domain.OrderStatusHistory{ID: uuid.New(), OrderID: order.ID, Status: domain.OrderCompleted, ChangedAt: testDate}).Error)

		stats, err := repo.AggregateDailyStats(ctx, testDate)
		require.NoError(t, err)
		assert.NotEmpty(t, stats)
		assert.Equal(t, 500.0, stats[0].GrossRevenue)
	})
}
