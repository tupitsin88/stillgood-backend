package main

import (
	"strings"
	"testing"
)

func TestMigrationsDirUsesConfiguredDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MIGRATIONS_DIR", dir)

	got, err := migrationsDir()
	if err != nil {
		t.Fatalf("migrationsDir returned error: %v", err)
	}
	if got != dir {
		t.Fatalf("migrationsDir() = %q, want %q", got, dir)
	}
}

func TestMigrationsDirRejectsMissingConfiguredDirectory(t *testing.T) {
	t.Setenv("MIGRATIONS_DIR", "/path/that/does/not/exist")

	if _, err := migrationsDir(); err == nil {
		t.Fatal("migrationsDir returned nil error for missing configured directory")
	}
}

func TestOpenDBRequiresEnvironment(t *testing.T) {
	for _, key := range []string{"DB_HOST", "DB_USER", "DB_PASSWORD", "DB_NAME", "DB_PORT"} {
		t.Setenv(key, "")
	}

	_, err := openDB()
	if err == nil {
		t.Fatal("openDB returned nil error without required environment")
	}
	if !strings.Contains(err.Error(), "DB_HOST is required") {
		t.Fatalf("openDB error = %q, want DB_HOST requirement", err.Error())
	}
}
