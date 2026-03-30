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
	CategoryBreakdown []CategoryStat `json:"categoryBreakdown"`
}

type CategoryStat struct {
	Name         string  `json:"name"`
	GrossRevenue float64 `json:"grossRevenue"`
}

func (s *AnalyticsService) GetPartnerAnalytics(ctx context.Context, restaurantID uuid.UUID, start, end time.Time) (AnalyticsSummary, []domain.DailyAnalytics, error) {
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
	for name, revenue := range categoryMap {
		summary.CategoryBreakdown = append(summary.CategoryBreakdown, CategoryStat{
			Name:         name,
			GrossRevenue: revenue,
		})
	}
	if summary.TotalBookings > 0 {
		summary.ConversionRate = (float64(summary.CompletedOrders) / float64(summary.TotalBookings)) * 100
	}
	return summary, dailyStats, nil
}
