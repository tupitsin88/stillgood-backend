package restaurants

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"kursach_backend/internal/domain"
	offerspkg "kursach_backend/internal/offers"
	"kursach_backend/pkg/postgres"

	"github.com/google/uuid"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type geoTestDBConfig struct {
	host     string
	user     string
	password string
	name     string
	port     string
}

func setupGeoTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	cfg := geoTestDBConfigFromEnv()
	ensureGeoTestDB(t, cfg)

	db, err := postgres.NewDB(cfg.dsn())
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)

	err = goose.Up(sqlDB, "../../migrations")
	require.NoError(t, err)

	require.NoError(t, db.Exec("TRUNCATE offers, restaurants, categories, users RESTART IDENTITY CASCADE").Error)

	return db
}

func geoTestDBConfigFromEnv() geoTestDBConfig {
	cfg := geoTestDBConfig{
		host:     geoEnvOrDefault("TEST_DB_HOST", geoEnvOrDefault("DB_HOST", "localhost")),
		user:     geoEnvOrDefault("TEST_DB_USER", geoEnvOrDefault("DB_USER", "postgres")),
		password: geoEnvOrDefault("TEST_DB_PASSWORD", geoEnvOrDefault("DB_PASSWORD", "hsefcsse243_secret_password_postgres")),
		name:     geoEnvOrDefault("TEST_GEO_DB_NAME", "foodsharing_geo_test_db"),
		port:     geoEnvOrDefault("TEST_DB_PORT", geoEnvOrDefault("DB_PORT", "5433")),
	}
	if cfg.host == "postgres" && cfg.port == "5432" && !geoRunningInDocker() {
		cfg.host = "localhost"
		cfg.port = "5433"
	}
	return cfg
}

func ensureGeoTestDB(t *testing.T, cfg geoTestDBConfig) {
	t.Helper()

	adminCfg := cfg
	adminCfg.name = geoEnvOrDefault("TEST_DB_ADMIN_NAME", "postgres")

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

	err = db.Exec("CREATE DATABASE " + geoQuoteIdentifier(cfg.name)).Error
	require.NoError(t, err)
}

func (c geoTestDBConfig) dsn() string {
	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		c.host,
		c.user,
		c.password,
		c.name,
		c.port,
	)
}

func geoEnvOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func geoQuoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func geoRunningInDocker() bool {
	_, err := os.Stat("/.dockerenv")
	return err == nil
}

func TestRestaurantRepositoryGetListUsesPostGISRadiusAndDistanceOrder(t *testing.T) {
	db := setupGeoTestDB(t)
	repo := NewRepository(db)

	partner := domain.User{
		ID:    uuid.New(),
		Email: "geo-partner@test.com",
		Role:  "PARTNER",
		Name:  "Test Partner",
	}
	require.NoError(t, db.Create(&partner).Error)
	center := domain.Restaurant{
		ID:        uuid.New(),
		PartnerID: partner.ID,
		Name:      "Center Restaurant",
		Latitude:  55.7558,
		Longitude: 37.6173,
		IsActive:  true,
	}
	near := domain.Restaurant{ID: uuid.New(), PartnerID: partner.ID, Name: "Near", Latitude: 55.7620, Longitude: 37.6200, IsActive: true}
	far := domain.Restaurant{ID: uuid.New(), PartnerID: partner.ID, Name: "Far", Latitude: 59.9343, Longitude: 30.3351, IsActive: true}
	inactive := domain.Restaurant{ID: uuid.New(), PartnerID: partner.ID, Name: "Inactive", Latitude: 55.7560, Longitude: 37.6175, IsActive: false}
	require.NoError(t, db.Create(&[]domain.Restaurant{center, near, far, inactive}).Error)

	lat := 55.7558
	lng := 37.6173
	radius := 5_000
	restaurants, total, err := repo.GetList(ListParams{
		Lat:    &lat,
		Lng:    &lng,
		Radius: &radius,
		Limit:  10,
		Offset: 0,
	})

	require.NoError(t, err)
	require.Len(t, restaurants, 2)
	assert.Equal(t, int64(2), total)
	assert.Equal(t, center.ID, restaurants[0].ID)
	assert.Equal(t, near.ID, restaurants[1].ID)
	require.NotNil(t, restaurants[0].DistanceMeters)
	require.NotNil(t, restaurants[1].DistanceMeters)
	assert.Equal(t, 0, *restaurants[0].DistanceMeters)
	assert.Greater(t, *restaurants[1].DistanceMeters, 0)
}

func TestOfferRepositoryGetPublicOffersUsesPostGISRadiusAndDistanceOrder(t *testing.T) {
	db := setupGeoTestDB(t)
	repo := offerspkg.NewOfferRepository(db)
	now := time.Now()
	partner := domain.User{ID: uuid.New(), Email: "offer-geo@test.com", Role: "PARTNER", Name: "Partner"}
	require.NoError(t, db.Create(&partner).Error)

	category := domain.Category{ID: uuid.New(), Name: "Geo Test"}
	require.NoError(t, db.Create(&category).Error)

	closestRestaurant := domain.Restaurant{
		ID:        uuid.New(),
		PartnerID: partner.ID,
		Name:      "Closest",
		Latitude:  55.7558,
		Longitude: 37.6173,
		IsActive:  true,
	}
	fartherRestaurant := domain.Restaurant{
		ID:        uuid.New(),
		PartnerID: partner.ID,
		Name:      "Farther",
		Latitude:  55.7680,
		Longitude: 37.6400,
		IsActive:  true,
	}
	outsideRadiusRestaurant := domain.Restaurant{
		ID:        uuid.New(),
		PartnerID: partner.ID,
		Name:      "Outside Radius",
		Latitude:  59.9343,
		Longitude: 30.3351,
		IsActive:  true,
	}
	inactiveRestaurant := domain.Restaurant{
		ID:        uuid.New(),
		PartnerID: partner.ID,
		Name:      "Inactive",
		Latitude:  55.7560,
		Longitude: 37.6175,
		IsActive:  false,
	}
	require.NoError(t, db.Create(&[]domain.Restaurant{closestRestaurant, fartherRestaurant, outsideRadiusRestaurant, inactiveRestaurant}).Error)

	offers := []domain.Offer{
		{
			ID:                uuid.New(),
			RestaurantID:      closestRestaurant.ID,
			CategoryID:        category.ID,
			Title:             "Closest Offer",
			Price:             100,
			OriginalPrice:     150,
			QuantityAvailable: 1,
			QuantityTotal:     1,
			PickupStart:       now.Add(time.Hour),
			PickupEnd:         now.Add(2 * time.Hour),
			IsActive:          true,
		},
		{
			ID:                uuid.New(),
			RestaurantID:      fartherRestaurant.ID,
			CategoryID:        category.ID,
			Title:             "Farther Offer",
			Price:             100,
			OriginalPrice:     150,
			QuantityAvailable: 1,
			QuantityTotal:     1,
			PickupStart:       now.Add(time.Hour),
			PickupEnd:         now.Add(2 * time.Hour),
			IsActive:          true,
		},
		{
			ID:                uuid.New(),
			RestaurantID:      outsideRadiusRestaurant.ID,
			CategoryID:        category.ID,
			Title:             "Outside Radius Offer",
			Price:             100,
			OriginalPrice:     150,
			QuantityAvailable: 1,
			QuantityTotal:     1,
			PickupStart:       now.Add(time.Hour),
			PickupEnd:         now.Add(2 * time.Hour),
			IsActive:          true,
		},
		{
			ID:                uuid.New(),
			RestaurantID:      inactiveRestaurant.ID,
			CategoryID:        category.ID,
			Title:             "Inactive Restaurant Offer",
			Price:             100,
			OriginalPrice:     150,
			QuantityAvailable: 1,
			QuantityTotal:     1,
			PickupStart:       now.Add(time.Hour),
			PickupEnd:         now.Add(2 * time.Hour),
			IsActive:          true,
		},
	}
	require.NoError(t, db.Create(&offers).Error)

	lat := 55.7558
	lng := 37.6173
	radius := 5_000
	got, total, err := repo.GetPublicOffers(context.Background(), offerspkg.FilterParams{
		Lat:    &lat,
		Lng:    &lng,
		Radius: &radius,
		SortBy: "distance",
		Limit:  10,
		Offset: 0,
	})

	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, int64(2), total)
	assert.Equal(t, "Closest Offer", got[0].Title)
	assert.Equal(t, "Farther Offer", got[1].Title)
	require.NotNil(t, got[0].Distance)
	require.NotNil(t, got[1].Distance)
	assert.Equal(t, 0, *got[0].Distance)
	assert.Greater(t, *got[1].Distance, 0)
}
