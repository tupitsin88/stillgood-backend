package orders

import (
	"context"
	"fmt"
	"kursach_backend/internal/domain"
	"kursach_backend/pkg/postgres"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type testDBConfig struct {
	host     string
	user     string
	password string
	name     string
	port     string
}

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	cfg := testDBConfigFromEnv()
	ensureTestDB(t, cfg)

	db, err := postgres.NewDB(cfg.dsn())
	if err != nil {
		t.Fatalf("DB connection failed (%s:%s/%s): %v", cfg.host, cfg.port, cfg.name, err)
	}

	err = db.AutoMigrate(
		&domain.User{},
		&domain.Restaurant{},
		&domain.Category{},
		&domain.Offer{},
		&domain.Order{},
		&domain.OrderStatusHistory{},
	)
	if err != nil {
		t.Fatalf("DB migration failed: %v", err)
	}

	return db
}

func testDBConfigFromEnv() testDBConfig {
	cfg := testDBConfig{
		host:     envOrDefault("TEST_DB_HOST", envOrDefault("DB_HOST", "localhost")),
		user:     envOrDefault("TEST_DB_USER", envOrDefault("DB_USER", "postgres")),
		password: envOrDefault("TEST_DB_PASSWORD", envOrDefault("DB_PASSWORD", "hsefcsse243_secret_password_postgres")),
		name:     envOrDefault("TEST_DB_NAME", "foodsharing_test_db"),
		port:     envOrDefault("TEST_DB_PORT", envOrDefault("DB_PORT", "5433")),
	}
	if cfg.host == "postgres" && cfg.port == "5432" && !runningInDocker() {
		cfg.host = "localhost"
		cfg.port = "5433"
	}
	return cfg
}

func ensureTestDB(t *testing.T, cfg testDBConfig) {
	t.Helper()

	adminCfg := cfg
	adminCfg.name = envOrDefault("TEST_DB_ADMIN_NAME", "postgres")

	db, err := postgres.NewDB(adminCfg.dsn())
	if err != nil {
		t.Fatalf("admin DB connection failed (%s:%s/%s): %v", adminCfg.host, adminCfg.port, adminCfg.name, err)
	}
	defer func() {
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	}()

	var exists bool
	err = db.Raw("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = ?)", cfg.name).Scan(&exists).Error
	require.NoError(t, err)
	if exists {
		return
	}

	err = db.Exec("CREATE DATABASE " + quoteIdentifier(cfg.name)).Error
	require.NoError(t, err)
}

func (c testDBConfig) dsn() string {
	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		c.host,
		c.user,
		c.password,
		c.name,
		c.port,
	)
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func quoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func runningInDocker() bool {
	_, err := os.Stat("/.dockerenv")
	return err == nil
}

func TestCreateOrder_Concurrency(t *testing.T) {
	db := setupTestDB(t)
	repo := NewOrderRepository(db)
	service := NewOrderService(repo, &LogNotificationProvider{})
	ctx := context.Background()
	testRunID := uuid.NewString()

	partner := &domain.User{
		ID:    uuid.New(),
		Email: fmt.Sprintf("orders-test-partner-%s@test.com", testRunID),
		Role:  "PARTNER",
	}
	require.NoError(t, db.Create(partner).Error)

	category := &domain.Category{ID: uuid.New(), Name: "Тестовая категория " + testRunID}
	require.NoError(t, db.Create(category).Error)

	restaurant := &domain.Restaurant{
		ID:        uuid.New(),
		Name:      "Test Food " + testRunID,
		PartnerID: partner.ID,
		Address:   "Test Street",
	}
	require.NoError(t, db.Create(restaurant).Error)

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
	require.NoError(t, err)
	const numReqs = 10
	userIDs := make([]uuid.UUID, numReqs)
	for i := 0; i < numReqs; i++ {
		userIDs[i] = uuid.New()
		user := &domain.User{
			ID:    userIDs[i],
			Email: fmt.Sprintf("orders-test-user-%s-%d@test.com", testRunID, i),
			Name:  fmt.Sprintf("User %d", i),
			Role:  "USER",
		}
		require.NoError(t, db.Create(user).Error)
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
