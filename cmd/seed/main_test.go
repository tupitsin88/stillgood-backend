package main

import (
	"math"
	"testing"
	"time"
)

func TestBuildDemoDataCreatesConsistentDataset(t *testing.T) {
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)

	data, err := buildDemoData(now)
	if err != nil {
		t.Fatalf("buildDemoData returned error: %v", err)
	}
	if len(data.users) == 0 || len(data.categories) == 0 || len(data.restaurants) == 0 || len(data.offers) == 0 || len(data.orders) == 0 {
		t.Fatalf("buildDemoData returned incomplete dataset: users=%d categories=%d restaurants=%d offers=%d orders=%d",
			len(data.users), len(data.categories), len(data.restaurants), len(data.offers), len(data.orders))
	}

	for _, restaurant := range data.restaurants {
		if restaurant.LogoURL == nil || *restaurant.LogoURL == "" {
			t.Fatalf("restaurant %s has empty logo URL", restaurant.ID)
		}
		if restaurant.CoverURL == nil || *restaurant.CoverURL == "" {
			t.Fatalf("restaurant %s has empty cover URL", restaurant.ID)
		}
	}

	for _, order := range data.orders {
		if order.OrderNumber == nil || *order.OrderNumber == "" {
			t.Fatalf("order %s has empty order number", order.ID)
		}
		wantFee := order.Amount * 0.15
		if math.Abs(order.ServiceFee-wantFee) > 0.0001 {
			t.Fatalf("order %s service fee = %v, want %v", order.ID, order.ServiceFee, wantFee)
		}
	}

	if len(data.analytics) == 0 {
		t.Fatal("buildDemoData returned no analytics rows")
	}
	analyticsDates := make(map[time.Time]struct{})
	for _, row := range data.analytics {
		analyticsDates[row.Date] = struct{}{}
		wantCreatedAt := analyticsGeneratedAt(row.Date)
		if !row.CreatedAt.Equal(wantCreatedAt) {
			t.Fatalf("analytics row %s created_at = %v, want %v", row.ID, row.CreatedAt, wantCreatedAt)
		}
	}
	if len(analyticsDates) != analyticsSeedDays {
		t.Fatalf("analytics rows cover %d dates, want %d", len(analyticsDates), analyticsSeedDays)
	}
}

func TestDayStartTruncatesToUTCDate(t *testing.T) {
	value := time.Date(2026, 5, 9, 18, 30, 15, 100, time.UTC)
	want := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)

	if got := dayStart(value); !got.Equal(want) {
		t.Fatalf("dayStart() = %v, want %v", got, want)
	}
}
