package analytics

import (
	"context"
	"kursach_backend/internal/domain"
	"log"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

type Repository interface {
	AggregateDailyStats(ctx context.Context, date time.Time) ([]domain.DailyAnalytics, error)
	SaveStats(ctx context.Context, stats []domain.DailyAnalytics) error
	GetStats(ctx context.Context, restaurantID uuid.UUID, start, end time.Time) ([]domain.DailyAnalytics, error)
	GetRestaurantByPartnerID(ctx context.Context, partnerID uuid.UUID) (*domain.Restaurant, error)
}

type AnalyticsService struct {
	repo Repository
}

func NewAnalyticsService(repo Repository) *AnalyticsService {
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
	c := cron.New(cron.WithLocation(time.Local))
	_, err := c.AddFunc("0 2 * * *", func() {
		s.RunDailyAggregation(context.Background())
	})
	if err != nil {
		log.Printf("[Analytics Cron] Error scheduling job: %v", err)
		return
	}
	c.Start()
	log.Printf("[Analytics Worker] Cron scheduler started. Next run at 02:00 daily.")
	<-ctx.Done()
	log.Printf("[Analytics Worker] Stopping scheduler...")
	c.Stop()
}

type AnalyticsSummary struct {
	TotalBookings     int            `json:"totalBookings"`
	CompletedOrders   int            `json:"completedOrders"`
	CancelledOrders   int            `json:"cancelledOrders"`
	ExpiredOrders     int            `json:"expiredOrders"`
	TotalRevenue      float64        `json:"totalRevenue"`
	ServiceFee        float64        `json:"serviceFee"`
	NetPayout         float64        `json:"netPayout"`
	ConversionRate    float64        `json:"conversionRate"`
	CancelRate        float64        `json:"cancelRate"`
	CategoryBreakdown []CategoryStat `json:"categoryBreakdown"`
}

type CategoryStat struct {
	Name         string  `json:"name"`
	TotalRevenue float64 `json:"totalRevenue"`
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
		summary.ExpiredOrders += day.ExpiredOrders
		summary.TotalRevenue += day.GrossRevenue
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
			TotalRevenue: revenue,
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
	sort.Slice(result, func(i, j int) bool {
		return result[i].Date.Before(result[j].Date)
	})
	return result
}
