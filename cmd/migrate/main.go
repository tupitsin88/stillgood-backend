package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"kursach_backend/pkg/postgres"

	"github.com/pressly/goose/v3"
	"gorm.io/gorm"
)

func main() {
	db, err := openDBWithRetry()
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("get sql.DB: %v", err)
	}
	defer sqlDB.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatalf("set goose dialect: %v", err)
	}

	dir, err := migrationsDir()
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Applying Goose migrations from %s...", dir)
	if err := goose.Up(sqlDB, dir); err != nil {
		log.Fatalf("apply migrations: %v", err)
	}
	log.Println("Goose migrations completed successfully")
}

func openDBWithRetry() (*gorm.DB, error) {
	var db *gorm.DB
	var err error
	for i := 0; i < 10; i++ {
		db, err = openDB()
		if err == nil {
			return db, nil
		}
		log.Printf("Database is not ready, retrying in 2 seconds... (%d/10)", i+1)
		time.Sleep(2 * time.Second)
	}
	return nil, err
}

func openDB() (*gorm.DB, error) {
	required := []string{"DB_HOST", "DB_USER", "DB_PASSWORD", "DB_NAME", "DB_PORT"}
	for _, key := range required {
		if os.Getenv(key) == "" {
			return nil, fmt.Errorf("%s is required", key)
		}
	}

	dsn := "host=" + os.Getenv("DB_HOST") +
		" user=" + os.Getenv("DB_USER") +
		" password=" + os.Getenv("DB_PASSWORD") +
		" dbname=" + os.Getenv("DB_NAME") +
		" port=" + os.Getenv("DB_PORT") +
		" sslmode=disable"

	return postgres.NewDB(dsn)
}

func migrationsDir() (string, error) {
	if dir := os.Getenv("MIGRATIONS_DIR"); dir != "" {
		if _, err := os.Stat(dir); err != nil {
			return "", fmt.Errorf("MIGRATIONS_DIR %q is not available: %w", dir, err)
		}
		return dir, nil
	}

	for _, dir := range []string{"migrations", "../../migrations"} {
		if _, err := os.Stat(dir); err == nil {
			return dir, nil
		}
	}

	return "", fmt.Errorf("migrations directory not found")
}
