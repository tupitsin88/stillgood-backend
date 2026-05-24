package main

import (
	"math"
	"testing"
	"time"

	"kursach_backend/internal/domain"
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
		if order.PaidAt != nil && (order.OrderNumber == nil || *order.OrderNumber == "") {
			t.Fatalf("order %s has empty order number", order.ID)
		}
		if order.PaidAt == nil && order.OrderNumber != nil {
			t.Fatalf("unpaid order %s has order number %q", order.ID, *order.OrderNumber)
		}
	}

	restByID := restaurantsByID(data.restaurants)
	offerByID := offersByID(data.offers)
	for _, order := range data.orders {
		offer := offerByID[order.OfferID]
		restaurant := restByID[offer.RestaurantID]
		if order.Status == domain.OrderCompleted {
			wantFee := order.Amount * restaurant.Commission / 100
			if math.Abs(order.ServiceFee-wantFee) > 0.0001 {
				t.Fatalf("order %s service fee = %v, want %v", order.ID, order.ServiceFee, wantFee)
			}
			if math.Abs(order.NetPayout-(order.Amount-wantFee)) > 0.0001 {
				t.Fatalf("order %s net payout = %v, want %v", order.ID, order.NetPayout, order.Amount-wantFee)
			}
		} else if order.ServiceFee != 0 || order.NetPayout != 0 {
			t.Fatalf("non-completed order %s has service fee/net payout: %v/%v", order.ID, order.ServiceFee, order.NetPayout)
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

	assertAnalyticsMatchesOrders(t, data)
	assertReviewsMatchCompletedOrders(t, data)
	assertPartnerStatuses(t, data)
}

func TestDayStartTruncatesToUTCDate(t *testing.T) {
	value := time.Date(2026, 5, 9, 18, 30, 15, 100, time.UTC)
	want := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)

	if got := dayStart(value); !got.Equal(want) {
		t.Fatalf("dayStart() = %v, want %v", got, want)
	}
}

func assertAnalyticsMatchesOrders(t *testing.T, data demoData) {
	t.Helper()

	var wantBookings, wantCompleted, wantCancelled, wantExpired int
	var wantRevenue, wantServiceFee, wantNetPayout float64
	for _, order := range data.orders {
		if order.Status != domain.OrderCreated {
			wantBookings++
		}
		if order.Status == domain.OrderCompleted {
			wantRevenue += order.Amount
			wantServiceFee += order.ServiceFee
			wantNetPayout += order.NetPayout
		}
	}
	for _, history := range data.histories {
		switch history.Status {
		case domain.OrderCompleted:
			wantCompleted++
		case domain.OrderCancelled:
			wantCancelled++
		}
	}
	for _, order := range data.orders {
		if order.CancellationReason != nil && *order.CancellationReason == "expired" {
			wantExpired++
		}
	}

	var gotBookings, gotCompleted, gotCancelled, gotExpired int
	var gotRevenue, gotServiceFee, gotNetPayout float64
	for _, row := range data.analytics {
		gotBookings += row.TotalBookings
		gotCompleted += row.CompletedOrders
		gotCancelled += row.CancelledOrders
		gotExpired += row.ExpiredOrders
		gotRevenue += row.GrossRevenue
		gotServiceFee += row.ServiceFee
		gotNetPayout += row.NetPayout
	}

	if gotBookings != wantBookings || gotCompleted != wantCompleted || gotCancelled != wantCancelled || gotExpired != wantExpired {
		t.Fatalf("analytics counts = bookings:%d completed:%d cancelled:%d expired:%d, want %d/%d/%d/%d",
			gotBookings, gotCompleted, gotCancelled, gotExpired, wantBookings, wantCompleted, wantCancelled, wantExpired)
	}
	if math.Abs(gotRevenue-wantRevenue) > 0.0001 || math.Abs(gotServiceFee-wantServiceFee) > 0.0001 || math.Abs(gotNetPayout-wantNetPayout) > 0.0001 {
		t.Fatalf("analytics money = revenue:%v fee:%v payout:%v, want %v/%v/%v",
			gotRevenue, gotServiceFee, gotNetPayout, wantRevenue, wantServiceFee, wantNetPayout)
	}
}

func assertReviewsMatchCompletedOrders(t *testing.T, data demoData) {
	t.Helper()

	orderByID := ordersByID(data.orders)
	for _, review := range data.reviews {
		order, ok := orderByID[review.OrderID]
		if !ok {
			t.Fatalf("review %s points to missing order %s", review.ID, review.OrderID)
		}
		if order.Status != domain.OrderCompleted {
			t.Fatalf("review %s points to non-completed order %s", review.ID, review.OrderID)
		}
	}

	var bakeryRatings []int
	for _, review := range data.reviews {
		for _, restaurant := range data.restaurants {
			if restaurant.ID == review.RestaurantID && restaurant.Name == "Хлеб и Кофе" {
				bakeryRatings = append(bakeryRatings, review.Rating)
			}
		}
	}
	if len(bakeryRatings) != 5 {
		t.Fatalf("bakery reviews = %d, want 5", len(bakeryRatings))
	}
	total := 0
	for _, rating := range bakeryRatings {
		total += rating
	}
	if got := float64(total) / float64(len(bakeryRatings)); math.Abs(got-4.6) > 0.0001 {
		t.Fatalf("bakery rating average = %v, want 4.6", got)
	}
}

func assertPartnerStatuses(t *testing.T, data demoData) {
	t.Helper()

	statusCounts := map[string]int{}
	admins := 0
	for _, user := range data.users {
		if user.Role == "ADMIN" {
			admins++
		}
		if user.Role == "PARTNER" {
			statusCounts[user.PartnerStatus]++
		}
	}
	if admins != 1 {
		t.Fatalf("admins = %d, want 1", admins)
	}
	if statusCounts["APPROVED"] < 2 || statusCounts["PENDING"] < 2 || statusCounts["REJECTED"] < 2 {
		t.Fatalf("partner statuses are not saturated enough: %#v", statusCounts)
	}
}
