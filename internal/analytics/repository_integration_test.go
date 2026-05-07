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
		host:     os.Getenv("DB_HOST"),
		user:     os.Getenv("DB_USER"),
		password: os.Getenv("DB_PASSWORD"),
		name:     os.Getenv("DB_NAME"),
		port:     os.Getenv("DB_PORT"),
	}

	if cfg.host == "" {
		cfg.host = "localhost"
	}
	if cfg.user == "" {
		cfg.user = "postgres"
	}
	if cfg.name == "" {
		cfg.name = "foodsharing_test_db"
	}
	if cfg.port == "" {
		if os.Getenv("GITHUB_ACTIONS") == "true" {
			cfg.port = "5432"
		} else {
			cfg.port = "5433"
		}
	}
	// Если пароль пустой и мы НЕ в CI — ставим твой локальный дефолт
	if cfg.password == "" && os.Getenv("GITHUB_ACTIONS") != "true" {
		cfg.password = "hsefcsse243_secret_password_postgres"
	}

	db, err := postgres.NewDB(cfg.dsn())
	require.NoError(t, err)

	// Блокировка для параллельных пакетов
	db.Exec("SELECT pg_advisory_lock(123456)")
	t.Cleanup(func() {
		db.Exec("SELECT pg_advisory_unlock(123456)")
	})

	// Очистка и миграции
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

	order := &domain.Order{ID: uuid.New(), OfferID: offer.ID, UserID: customer.ID, Amount: 500, Status: domain.OrderCompleted, CreatedAt: testDate}
	require.NoError(t, db.Create(order).Error)
	require.NoError(t, db.Create(&domain.OrderStatusHistory{ID: uuid.New(), OrderID: order.ID, Status: domain.OrderCompleted, ChangedAt: testDate}).Error)

	t.Run("Aggregate", func(t *testing.T) {
		stats, err := repo.AggregateDailyStats(ctx, testDate)
		require.NoError(t, err)
		require.NotEmpty(t, stats)
		assert.Equal(t, 500.0, stats[0].GrossRevenue)
	})

	t.Run("SaveStats", func(t *testing.T) {
		err := repo.SaveStats(ctx, []domain.DailyAnalytics{{
			RestaurantID: restaurant.ID, Date: testDate, CategoryName: category.Name, TotalBookings: 1,
		}})
		assert.NoError(t, err)
	})
}
