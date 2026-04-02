package analytics

import (
	"context"
	"kursach_backend/internal/domain"
	"log"
	"time"

	"github.com/google/uuid"
)

type AnalyticsService struct {
	repo *AnalyticsRepository
}

func NewAnalyticsService(repo *AnalyticsRepository) *AnalyticsService {
	return &AnalyticsService{repo: repo}
}

func (s *AnalyticsService) RunDailyAggregation(ctx context.Context) {
	yesterday := time.Now().AddDate(0, 0, -1)
	log.Printf("[Analytics] Starting aggregation for date: %s", yesterday.Format("2006-01-02"))
	stats, err := s.repo.AggregateDailyStats(ctx, yesterday)
	if err != nil {
		log.Printf("[Analytics] Error aggregating stats: %v", err)
		return
	}
	if len(stats) == 0 {
		log.Printf("[Analytics] No data to aggregate for %s", yesterday.Format("2006-01-02"))
		return
	}
	err = s.repo.SaveStats(ctx, stats)
	if err != nil {
		log.Printf("[Analytics] Error saving aggregated stats: %v", err)
		return
	}
	log.Printf("[Analytics] Successfully aggregated data for %d restaurants", len(stats))
}

func (s *AnalyticsService) StartAnalyticsWorker(ctx context.Context) {
	for {
		now := time.Now()
		nextRun := time.Date(now.Year(), now.Month(), now.Day(), 2, 0, 0, 0, now.Location())
		if now.After(nextRun) {
			nextRun = nextRun.AddDate(0, 0, 1)
		}
		durationUntilNextRun := time.Until(nextRun)
		log.Printf("[Analytics Worker] Next run scheduled at %s (in %v)", nextRun.Format("15:04:05"), durationUntilNextRun)
		select {
		case <-ctx.Done():
			return
		case <-time.After(durationUntilNextRun):
			s.RunDailyAggregation(ctx)
		}
	}
}

type AnalyticsSummary struct {
	TotalBookings     int            `json:"totalBookings"`
	CompletedOrders   int            `json:"completedOrders"`
	CancelledOrders   int            `json:"cancelledOrders"`
	GrossRevenue      float64        `json:"grossRevenue"`
	ServiceFee        float64        `json:"serviceFee"`
	NetPayout         float64        `json:"netPayout"`
	ConversionRate    float64        `json:"conversionRate"`
	CancelRate        float64        `json:"cancelRate"`
	CategoryBreakdown []CategoryStat `json:"categoryBreakdown"`
}

type CategoryStat struct {
	Name         string  `json:"name"`
	GrossRevenue float64 `json:"grossRevenue"`
}

func (s *AnalyticsService) GetPartnerAnalytics(ctx context.Context, restaurantID uuid.UUID, start, end time.Time, groupBy string) (AnalyticsSummary, []domain.DailyAnalytics, error) {
	dailyStats, err := s.repo.GetStats(ctx, restaurantID, start, end)
	if err != nil {
		return AnalyticsSummary{}, nil, err
	}

	var summary AnalyticsSummary
	categoryMap := make(map[string]float64)
	for _, day := range dailyStats {
		summary.TotalBookings += day.TotalBookings
		summary.CompletedOrders += day.CompletedOrders
		summary.CancelledOrders += day.CancelledOrders
		summary.GrossRevenue += day.GrossRevenue
		summary.ServiceFee += day.ServiceFee
		summary.NetPayout += day.NetPayout
		if day.CategoryName != "" {
			categoryMap[day.CategoryName] += day.GrossRevenue
		}
	}
	if summary.TotalBookings > 0 {
		summary.ConversionRate = (float64(summary.CompletedOrders) / float64(summary.TotalBookings)) * 100
		summary.CancelRate = (float64(summary.CancelledOrders) / float64(summary.TotalBookings)) * 100
	}

	for name, revenue := range categoryMap {
		summary.CategoryBreakdown = append(summary.CategoryBreakdown, CategoryStat{
			Name:         name,
			GrossRevenue: revenue,
		})
	}
	if groupBy == "week" || groupBy == "month" {
		return summary, s.groupStats(dailyStats, groupBy), nil
	}
	return summary, dailyStats, nil
}

func (s *AnalyticsService) groupStats(stats []domain.DailyAnalytics, interval string) []domain.DailyAnalytics {
	if interval == "day" || len(stats) == 0 {
		return stats
	}
	groupedMap := make(map[time.Time]*domain.DailyAnalytics)
	for _, day := range stats {
		var periodKey time.Time
		if interval == "week" {
			weekday := int(day.Date.Weekday())
			if weekday == 0 {
				weekday = 7
			}
			periodKey = day.Date.AddDate(0, 0, -(weekday - 1))
		} else if interval == "month" {
			periodKey = time.Date(day.Date.Year(), day.Date.Month(), 1, 0, 0, 0, 0, day.Date.Location())
		}

		if _, exists := groupedMap[periodKey]; !exists {
			groupedMap[periodKey] = &domain.DailyAnalytics{
				ID:           uuid.New(),
				Date:         periodKey,
				RestaurantID: day.RestaurantID,
				CategoryName: "Все категории",
				CreatedAt:    time.Now(),
			}
		}
		g := groupedMap[periodKey]
		g.TotalBookings += day.TotalBookings
		g.CompletedOrders += day.CompletedOrders
		g.CancelledOrders += day.CancelledOrders
		g.GrossRevenue += day.GrossRevenue
		g.ServiceFee += day.ServiceFee
		g.NetPayout += day.NetPayout
	}
	var result []domain.DailyAnalytics
	for _, v := range groupedMap {
		result = append(result, *v)
	}
	return result
}
