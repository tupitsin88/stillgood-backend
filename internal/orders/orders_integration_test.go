package orders

import (
	"context"
	"fmt"
	"kursach_backend/internal/domain"
	"kursach_backend/internal/notifications"
	"kursach_backend/pkg/postgres"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pressly/goose/v3"
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
		t.Fatalf("DB connection failed: %v", err)
	}
	err = db.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public;").Error
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	if err := goose.Up(sqlDB, "../../migrations"); err != nil {
		t.Fatalf("Goose migration failed: %v", err)
	}

	return db
}

func newTestOrderService(db *gorm.DB, repo *OrderRepository) *OrderService {
	notificationRepo := notifications.NewRepository(db)
	notificationService := notifications.NewService(notificationRepo, notifications.LogPushProvider{})
	return NewOrderService(repo, notificationService)
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
	service := newTestOrderService(db, repo)
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

func TestOrder_StateTransitions(t *testing.T) {
	db := setupTestDB(t)
	repo := NewOrderRepository(db)
	service := newTestOrderService(db, repo)
	ctx := context.Background()
	testRunID := uuid.NewString()

	userID := uuid.New()
	require.NoError(t, db.Create(&domain.User{ID: userID, Email: "user-" + testRunID + "@t.com", Role: "USER"}).Error)

	partnerID := uuid.New()
	require.NoError(t, db.Create(&domain.User{ID: partnerID, Email: "p-" + testRunID + "@t.com", Role: "PARTNER"}).Error)

	catID := uuid.New()
	require.NoError(t, db.Create(&domain.Category{ID: catID, Name: "Cat " + testRunID}).Error)

	restID := uuid.New()
	require.NoError(t, db.Create(&domain.Restaurant{ID: restID, PartnerID: partnerID, Name: "Rest"}).Error)

	offerID := uuid.New()
	require.NoError(t, db.Create(&domain.Offer{
		ID: offerID, RestaurantID: restID, CategoryID: catID, Title: "Food", Price: 100, IsActive: true,
	}).Error)

	orderID := uuid.New()
	cancelledOrder := &domain.Order{
		ID:      orderID,
		UserID:  userID,
		OfferID: offerID,
		Status:  domain.OrderCancelled,
		Amount:  100,
	}
	require.NoError(t, db.Create(cancelledOrder).Error)

	_, err := service.PayOrder(ctx, orderID, userID)
	assert.Error(t, err)
	assert.Equal(t, "INVALID_ORDER_STATUS", err.Error(), "Нельзя оплатить уже отмененный заказ")
}

func TestOrder_PartnerSecurity(t *testing.T) {
	db := setupTestDB(t)
	repo := NewOrderRepository(db)
	service := newTestOrderService(db, repo)
	ctx := context.Background()
	testRunID := uuid.NewString()

	catID := uuid.New()
	require.NoError(t, db.Create(&domain.Category{ID: catID, Name: "Secure Cat " + testRunID}).Error)

	userID := uuid.New()
	require.NoError(t, db.Create(&domain.User{ID: userID, Email: "c-" + testRunID + "@t.com", Role: "USER"}).Error)

	p1 := &domain.User{ID: uuid.New(), Email: "p1-" + testRunID + "@t.com", Role: "PARTNER"}
	p2 := &domain.User{ID: uuid.New(), Email: "p2-" + testRunID + "@t.com", Role: "PARTNER"}
	require.NoError(t, db.Create(p1).Error)
	require.NoError(t, db.Create(p2).Error)

	r1ID, r2ID := uuid.New(), uuid.New()
	require.NoError(t, db.Create(&domain.Restaurant{ID: r1ID, PartnerID: p1.ID, Name: "Rest 1"}).Error)
	require.NoError(t, db.Create(&domain.Restaurant{ID: r2ID, PartnerID: p2.ID, Name: "Rest 2"}).Error)

	offerID := uuid.New()
	require.NoError(t, db.Create(&domain.Offer{
		ID: offerID, RestaurantID: r1ID, CategoryID: catID, Title: "Еда", IsActive: true,
	}).Error)

	orderID := uuid.New()
	require.NoError(t, db.Create(&domain.Order{
		ID: orderID, UserID: userID, OfferID: offerID, Status: domain.OrderPaid,
	}).Error)

	order, err := service.GetPartnerOrderByID(ctx, orderID, r1ID)
	require.NoError(t, err)
	assert.Equal(t, orderID, order.ID)

	_, err = service.GetPartnerOrderByID(ctx, orderID, r2ID)
	assert.Error(t, err)
	assert.Equal(t, "unauthorized", err.Error(), "Партнер не должен видеть детали чужого заказа")

	_, err = service.CompleteOrder(ctx, orderID, r2ID)
	assert.Error(t, err)
	assert.Equal(t, "unauthorized", err.Error(), "Партнер не должен иметь доступа к чужим заказам")
}

func TestOrder_ForbiddenComplete(t *testing.T) {
	db := setupTestDB(t)
	repo := NewOrderRepository(db)
	service := newTestOrderService(db, repo)
	ctx := context.Background()
	testRunID := uuid.NewString()

	catID := uuid.New()
	require.NoError(t, db.Create(&domain.Category{ID: catID, Name: "Forbidden Cat " + testRunID}).Error)

	partner := &domain.User{ID: uuid.New(), Email: "partner-" + testRunID + "@t.com", Role: "PARTNER"}
	require.NoError(t, db.Create(partner).Error)
	restID := uuid.New()
	require.NoError(t, db.Create(&domain.Restaurant{
		ID:        restID,
		PartnerID: partner.ID,
		Name:      "Test Rest " + testRunID,
	}).Error)

	userID := uuid.New()
	require.NoError(t, db.Create(&domain.User{ID: userID, Email: "customer-" + testRunID + "@t.com", Role: "USER"}).Error)

	offerID := uuid.New()
	require.NoError(t, db.Create(&domain.Offer{
		ID: offerID, RestaurantID: restID, CategoryID: catID, Title: "Free Food?", IsActive: true,
	}).Error)

	orderID := uuid.New()
	require.NoError(t, db.Create(&domain.Order{
		ID:      orderID,
		UserID:  userID,
		OfferID: offerID,
		Status:  domain.OrderCreated,
	}).Error)

	_, err := service.CompleteOrder(ctx, orderID, restID)
	assert.Error(t, err)
	assert.Equal(t, "INVALID_ORDER_STATUS", err.Error(), "Нельзя завершить неоплаченный заказ")
}

func TestOrder_PayAfterCancel(t *testing.T) {
	db := setupTestDB(t)
	repo := NewOrderRepository(db)
	service := newTestOrderService(db, repo)
	ctx := context.Background()
	testRunID := uuid.NewString()

	userID := uuid.New()
	require.NoError(t, db.Create(&domain.User{ID: userID, Email: "late_payer-" + testRunID + "@t.com", Role: "USER"}).Error)

	catID := uuid.New()
	require.NoError(t, db.Create(&domain.Category{ID: catID, Name: "Cancel Cat " + testRunID}).Error)

	partner := &domain.User{ID: uuid.New(), Email: "partner-" + testRunID + "@t.com", Role: "PARTNER"}
	require.NoError(t, db.Create(partner).Error)
	restID := uuid.New()
	require.NoError(t, db.Create(&domain.Restaurant{
		ID:        restID,
		PartnerID: partner.ID,
		Name:      "Test Rest " + testRunID,
	}).Error)

	offerID := uuid.New()
	require.NoError(t, db.Create(&domain.Offer{
		ID: offerID, RestaurantID: restID, CategoryID: catID, Title: "Old Food", IsActive: true,
	}).Error)

	orderID := uuid.New()
	require.NoError(t, db.Create(&domain.Order{
		ID:      orderID,
		UserID:  userID,
		OfferID: offerID,
		Status:  domain.OrderCancelled,
	}).Error)

	_, err := service.PayOrder(ctx, orderID, userID)

	assert.Error(t, err)
	assert.Equal(t, "INVALID_ORDER_STATUS", err.Error(), "Нельзя оплатить отмененный заказ")
}

func TestOrder_OfferAutoDeactivation(t *testing.T) {
	db := setupTestDB(t)
	repo := NewOrderRepository(db)
	service := newTestOrderService(db, repo)
	ctx := context.Background()
	testRunID := uuid.NewString()

	catID := uuid.New()
	require.NoError(t, db.Create(&domain.Category{ID: catID, Name: "Deactivation Cat " + testRunID}).Error)

	partner := &domain.User{ID: uuid.New(), Email: "partner-" + testRunID + "@t.com", Role: "PARTNER"}
	require.NoError(t, db.Create(partner).Error)
	restID := uuid.New()
	require.NoError(t, db.Create(&domain.Restaurant{
		ID:        restID,
		PartnerID: partner.ID,
		Name:      "Test Rest " + testRunID,
	}).Error)

	userID := uuid.New()
	require.NoError(t, db.Create(&domain.User{ID: userID, Email: "auto-buyer-" + testRunID + "@t.com"}).Error)

	offerID := uuid.New()
	require.NoError(t, db.Create(&domain.Offer{
		ID:                offerID,
		RestaurantID:      restID,
		CategoryID:        catID,
		Title:             "One Shot Food",
		QuantityAvailable: 1,
		QuantityTotal:     1,
		IsActive:          true,
		Price:             100,
		PickupEnd:         time.Now().Add(time.Hour),
	}).Error)

	_, err := service.CreateOrder(ctx, userID, CreateOrderRequest{OfferID: offerID.String()})
	require.NoError(t, err)

	var updatedOffer domain.Offer
	db.First(&updatedOffer, "id = ?", offerID)
	assert.False(t, updatedOffer.IsActive, "Оффер должен стать неактивным при остатке 0")
	assert.Equal(t, 0, updatedOffer.QuantityAvailable)
}

func TestOrder_CancellationWindow(t *testing.T) {
	db := setupTestDB(t)
	repo := NewOrderRepository(db)
	service := newTestOrderService(db, repo)
	ctx := context.Background()
	testRunID := uuid.NewString()

	catID := uuid.New()
	require.NoError(t, db.Create(&domain.Category{ID: catID, Name: "Refund Cat " + testRunID}).Error)

	partner := &domain.User{ID: uuid.New(), Email: "partner-" + testRunID + "@t.com", Role: "PARTNER"}
	require.NoError(t, db.Create(partner).Error)
	restID := uuid.New()
	require.NoError(t, db.Create(&domain.Restaurant{
		ID:        restID,
		PartnerID: partner.ID,
		Name:      "Test Rest " + testRunID,
	}).Error)

	userID := uuid.New()
	require.NoError(t, db.Create(&domain.User{ID: userID, Email: "refunder-" + testRunID + "@t.com"}).Error)

	offerEarlyID := uuid.New()
	require.NoError(t, db.Create(&domain.Offer{
		ID: offerEarlyID, RestaurantID: restID, CategoryID: catID,
		PickupStart: time.Now().Add(2 * time.Hour),
	}).Error)

	orderEarlyID := uuid.New()
	require.NoError(t, db.Create(&domain.Order{
		ID: orderEarlyID, UserID: userID, OfferID: offerEarlyID, Status: domain.OrderPaid, Amount: 500,
	}).Error)

	_, refund, err := service.CancelOrder(ctx, orderEarlyID, userID, "USER", "Change of plans")
	assert.NoError(t, err)
	assert.Equal(t, 500.0, refund, "Должен быть возврат за 2 часа")

	offerLateID := uuid.New()
	require.NoError(t, db.Create(&domain.Offer{
		ID: offerLateID, RestaurantID: restID, CategoryID: catID,
		PickupStart: time.Now().Add(30 * time.Minute),
	}).Error)

	orderLateID := uuid.New()
	require.NoError(t, db.Create(&domain.Order{
		ID: orderLateID, UserID: userID, OfferID: offerLateID, Status: domain.OrderPaid, Amount: 500,
	}).Error)

	_, _, err = service.CancelOrder(ctx, orderLateID, userID, "USER", "Too late")
	assert.Error(t, err)
	assert.Equal(t, "CANCELLATION_WINDOW_CLOSED", err.Error())
}

func TestOrder_PaymentExpiration(t *testing.T) {
	db := setupTestDB(t)
	repo := NewOrderRepository(db)
	service := newTestOrderService(db, repo)
	ctx := context.Background()
	testRunID := uuid.NewString()

	catID := uuid.New()
	require.NoError(t, db.Create(&domain.Category{ID: catID, Name: "Exp Cat " + testRunID}).Error)

	partner := &domain.User{ID: uuid.New(), Email: "partner-" + testRunID + "@t.com", Role: "PARTNER"}
	require.NoError(t, db.Create(partner).Error)
	restID := uuid.New()
	require.NoError(t, db.Create(&domain.Restaurant{
		ID:        restID,
		PartnerID: partner.ID,
		Name:      "Test Rest " + testRunID,
	}).Error)

	offerID := uuid.New()
	require.NoError(t, db.Create(&domain.Offer{ID: offerID, RestaurantID: restID, CategoryID: catID, Title: "Old Food"}).Error)

	userID := uuid.New()
	require.NoError(t, db.Create(&domain.User{ID: userID, Email: "late-payer-" + testRunID + "@t.com"}).Error)

	pastTime := time.Now().Add(-5 * time.Minute)
	orderID := uuid.New()
	require.NoError(t, db.Create(&domain.Order{
		ID:        orderID,
		UserID:    userID,
		OfferID:   offerID,
		Status:    domain.OrderCreated,
		ExpiresAt: &pastTime,
	}).Error)

	_, err := service.PayOrder(ctx, orderID, userID)

	assert.Error(t, err)
	assert.Equal(t, "ORDER_EXPIRED", err.Error(), "Нельзя оплатить заказ после истечения времени брони")
}
