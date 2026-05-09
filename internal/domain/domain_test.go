package domain

import "testing"

func TestOrderStatusValues(t *testing.T) {
	cases := map[OrderStatus]string{
		OrderCreated:   "CREATED",
		OrderPaid:      "PAID",
		OrderCompleted: "COMPLETED",
		OrderCancelled: "CANCELLED",
	}

	for status, want := range cases {
		if string(status) != want {
			t.Fatalf("status %q = %q, want %q", want, status, want)
		}
	}
}

func TestRefreshSessionTableName(t *testing.T) {
	if got := (RefreshSession{}).TableName(); got != "refresh_sessions" {
		t.Fatalf("RefreshSession.TableName() = %q, want %q", got, "refresh_sessions")
	}
}
