package postgres

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type invalidRestaurantCoordinate struct {
	ID        string
	Latitude  *float64
	Longitude *float64
}

func EnsurePostGIS(db *gorm.DB) error {
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS postgis").Error; err != nil {
		return fmt.Errorf("ensure postgis extension: %w", err)
	}
	return nil
}

func EnsureRestaurantGeoLayer(db *gorm.DB) error {
	if err := ensureRestaurantCoordinatesValid(db); err != nil {
		return err
	}
	if err := ensureRestaurantCoordinateConstraint(db, "restaurants_latitude_range", "latitude IS NOT NULL AND latitude >= -90 AND latitude <= 90"); err != nil {
		return err
	}
	if err := ensureRestaurantCoordinateConstraint(db, "restaurants_longitude_range", "longitude IS NOT NULL AND longitude >= -180 AND longitude <= 180"); err != nil {
		return err
	}
	if err := db.Exec(`
		ALTER TABLE restaurants
		ADD COLUMN IF NOT EXISTS location geography(Point, 4326)
		GENERATED ALWAYS AS (
			ST_SetSRID(ST_MakePoint(longitude, latitude), 4326)::geography
		) STORED
	`).Error; err != nil {
		return fmt.Errorf("ensure restaurants.location column: %w", err)
	}
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_restaurants_location_geog
		ON restaurants
		USING GIST (location)
	`).Error; err != nil {
		return fmt.Errorf("ensure restaurants.location index: %w", err)
	}
	return nil
}

func ensureRestaurantCoordinatesValid(db *gorm.DB) error {
	var rows []invalidRestaurantCoordinate
	if err := db.Raw(`
		SELECT id::text AS id, latitude, longitude
		FROM restaurants
		WHERE latitude IS NULL
			OR longitude IS NULL
			OR latitude < -90
			OR latitude > 90
			OR longitude < -180
			OR longitude > 180
		LIMIT 10
	`).Scan(&rows).Error; err != nil {
		return fmt.Errorf("validate restaurant coordinates: %w", err)
	}
	if len(rows) == 0 {
		return nil
	}

	details := make([]string, 0, len(rows))
	for _, row := range rows {
		details = append(details, fmt.Sprintf("id=%s latitude=%s longitude=%s", row.ID, formatOptionalFloat(row.Latitude), formatOptionalFloat(row.Longitude)))
	}
	return fmt.Errorf("invalid restaurant coordinates found before PostGIS migration: %s", strings.Join(details, "; "))
}

func ensureRestaurantCoordinateConstraint(db *gorm.DB, name, expression string) error {
	var exists bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1
			FROM pg_constraint
			WHERE conname = ?
				AND conrelid = 'restaurants'::regclass
		)
	`, name).Scan(&exists).Error; err != nil {
		return fmt.Errorf("check %s constraint: %w", name, err)
	}
	if exists {
		return nil
	}

	if err := db.Exec(fmt.Sprintf("ALTER TABLE restaurants ADD CONSTRAINT %s CHECK (%s)", name, expression)).Error; err != nil {
		return fmt.Errorf("add %s constraint: %w", name, err)
	}
	return nil
}

func formatOptionalFloat(value *float64) string {
	if value == nil {
		return "NULL"
	}
	return fmt.Sprintf("%g", *value)
}
