package postgres

import "testing"

func TestNewDBRejectsInvalidDSN(t *testing.T) {
	if _, err := NewDB("host=localhost port=not-a-number"); err == nil {
		t.Fatal("NewDB returned nil error for invalid DSN")
	}
}
