package analytics

import (
	"context"
	"testing"
	"time"

	"kursach_backend/internal/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type analyticsRepoStub struct {
	stats []domain.DailyAnalytics
	rest  *domain.Restaurant
	err   error
}

func (s *analyticsRepoStub) GetStats(ctx context.Context, id uuid.UUID, start, end time.Time) ([]domain.DailyAnalytics, error) {
	return s.stats, s.err
}

func (s *analyticsRepoStub) GetRestaurantByPartnerID(ctx context.Context, id uuid.UUID) (*domain.Restaurant, error) {
	return s.rest, s.err
}
func (s *analyticsRepoStub) AggregateDailyStats(ctx context.Context, date time.Time) ([]domain.DailyAnalytics, error) {
	return nil, nil
}

func (s *analyticsRepoStub) SaveStats(ctx context.Context, stats []domain.DailyAnalytics) error {
	return nil
}
func TestGetPartnerAnalytics_Calculations(t *testing.T) {
	restID := uuid.New()
	now := time.Now()

	testStats := []domain.DailyAnalytics{
		{
			TotalBookings:   10,
			CompletedOrders: 8,
			CancelledOrders: 1,
			GrossRevenue:    1000,
			ServiceFee:      150,
			NetPayout:       850,
			CategoryName:    "Пицца",
		},
		{
			TotalBookings:   5,
			CompletedOrders: 2,
			CancelledOrders: 2,
			GrossRevenue:    500,
			ServiceFee:      75,
			NetPayout:       425,
			CategoryName:    "Суши",
		},
	}

	t.Run("Correctly aggregates totals and rates", func(t *testing.T) {
		repo := &analyticsRepoStub{stats: testStats}
		service := NewAnalyticsService(repo)

		summary, _, err := service.GetPartnerAnalytics(context.Background(), restID, now, now, "day")

		require.NoError(t, err)
		assert.Equal(t, 15, summary.TotalBookings)
		assert.Equal(t, 10, summary.CompletedOrders)
		assert.Equal(t, 1500.0, summary.TotalRevenue)
		assert.InDelta(t, 66.66, summary.ConversionRate, 0.01)
		assert.Equal(t, 20.0, summary.CancelRate)
	})

	t.Run("Handles zero bookings without division by zero", func(t *testing.T) {
		repo := &analyticsRepoStub{stats: []domain.DailyAnalytics{}}
		service := NewAnalyticsService(repo)
		summary, _, err := service.GetPartnerAnalytics(context.Background(), restID, now, now, "day")
		require.NoError(t, err)
		assert.Equal(t, 0.0, summary.ConversionRate)
	})
}

func TestGroupStats_Logic(t *testing.T) {
	service := NewAnalyticsService(nil)
	date1 := time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)
	date2 := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	stats := []domain.DailyAnalytics{
		{Date: date1, TotalBookings: 10, GrossRevenue: 100},
		{Date: date2, TotalBookings: 5, GrossRevenue: 50},
	}

	t.Run("Groups by week correctly", func(t *testing.T) {
		grouped := service.groupStats(stats, "week")

		require.Len(t, grouped, 1)
		assert.Equal(t, 15, grouped[0].TotalBookings)
		assert.Equal(t, 150.0, grouped[0].GrossRevenue)
		assert.Equal(t, date1.Year(), grouped[0].Date.Year())
		assert.Equal(t, date1.Day(), grouped[0].Date.Day())
	})
}

func TestGroupStats_MonthLogic(t *testing.T) {
	service := NewAnalyticsService(nil)
	date1 := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	date2 := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)

	stats := []domain.DailyAnalytics{
		{Date: date1, TotalBookings: 10, GrossRevenue: 100},
		{Date: date2, TotalBookings: 5, GrossRevenue: 50},
	}

	t.Run("Groups by month correctly", func(t *testing.T) {
		grouped := service.groupStats(stats, "month")

		require.Len(t, grouped, 1)
		assert.Equal(t, 15, grouped[0].TotalBookings)
		assert.Equal(t, 150.0, grouped[0].GrossRevenue)
		assert.Equal(t, 1, grouped[0].Date.Day())
		assert.Equal(t, time.Month(5), grouped[0].Date.Month())
	})
}

func TestRunDailyAggregation_NoData(t *testing.T) {
	repo := &analyticsRepoStub{stats: nil}
	service := NewAnalyticsService(repo)

	t.Run("Does not panic when no daily stats found", func(t *testing.T) {
		assert.NotPanics(t, func() {
			service.RunDailyAggregation(context.Background())
		})
	})
}
