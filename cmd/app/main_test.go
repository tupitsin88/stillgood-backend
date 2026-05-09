package main

import (
	"testing"
	"time"
)

func TestDurationFromEnvUsesFallback(t *testing.T) {
	const key = "TEST_ACCESS_TOKEN_TTL"
	fallback := 15 * time.Minute
	t.Setenv(key, "")

	got, err := durationFromEnv(key, fallback)
	if err != nil {
		t.Fatalf("durationFromEnv returned error: %v", err)
	}
	if got != fallback {
		t.Fatalf("durationFromEnv() = %v, want %v", got, fallback)
	}
}

func TestDurationFromEnvParsesValue(t *testing.T) {
	const key = "TEST_ACCESS_TOKEN_TTL"
	t.Setenv(key, "45m")

	got, err := durationFromEnv(key, time.Minute)
	if err != nil {
		t.Fatalf("durationFromEnv returned error: %v", err)
	}
	if got != 45*time.Minute {
		t.Fatalf("durationFromEnv() = %v, want %v", got, 45*time.Minute)
	}
}

func TestDurationFromEnvRejectsInvalidValue(t *testing.T) {
	const key = "TEST_ACCESS_TOKEN_TTL"
	t.Setenv(key, "bad-duration")

	if _, err := durationFromEnv(key, time.Minute); err == nil {
		t.Fatal("durationFromEnv returned nil error for invalid duration")
	}
}
