package analytics

import (
	"context"
	"kursach_backend/internal/domain"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AnalyticsRepository struct {
	db *gorm.DB
}

func NewAnalyticsRepository(db *gorm.DB) *AnalyticsRepository {
	return &AnalyticsRepository{db: db}
}

func (r *AnalyticsRepository) AggregateDailyStats(ctx context.Context, date time.Time) ([]domain.DailyAnalytics, error) {
	var results []domain.DailyAnalytics
	dateStr := date.Format("2006-01-02")
	query := `
		SELECT 
			off.restaurant_id,
			c.name as category_name,
			COUNT(o.id) as total_bookings,
			COUNT(o.id) FILTER (WHERE o.status = 'COMPLETED') as completed_orders,
			COUNT(o.id) FILTER (WHERE o.status = 'CANCELLED') as cancelled_orders,
			COALESCE(SUM(o.amount) FILTER (WHERE o.status = 'COMPLETED'), 0) as gross_revenue
		FROM orders o
		JOIN offers off ON o.offer_id = off.id
		JOIN categories c ON off.category_id = c.id
		WHERE o.created_at::date = ?::date
		GROUP BY off.restaurant_id, c.name
	`
	type aggResult struct {
		RestaurantID    uuid.UUID
		CategoryName    string
		TotalBookings   int
		CompletedOrders int
		CancelledOrders int
		GrossRevenue    float64
	}
	var rawStats []aggResult
	err := r.db.WithContext(ctx).Raw(query, dateStr).Scan(&rawStats).Error
	if err != nil {
		return nil, err
	}
	for _, s := range rawStats {
		fee := s.GrossRevenue * 0.15
		results = append(results, domain.DailyAnalytics{
			ID:              uuid.New(),
			RestaurantID:    s.RestaurantID,
			Date:            date,
			CategoryName:    s.CategoryName,
			TotalBookings:   s.TotalBookings,
			CompletedOrders: s.CompletedOrders,
			CancelledOrders: s.CancelledOrders,
			GrossRevenue:    s.GrossRevenue,
			ServiceFee:      fee,
			NetPayout:       s.GrossRevenue - fee,
			CreatedAt:       time.Now(),
		})
	}

	return results, nil
}

func (r *AnalyticsRepository) SaveStats(ctx context.Context, stats []domain.DailyAnalytics) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "restaurant_id"}, {Name: "date"}, {Name: "category_name"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"total_bookings", "completed_orders", "cancelled_orders",
			"gross_revenue", "service_fee", "net_payout",
		}),
	}).Create(&stats).Error
}

func (r *AnalyticsRepository) GetStats(ctx context.Context, restaurantID uuid.UUID, start, end time.Time) ([]domain.DailyAnalytics, error) {
	var stats []domain.DailyAnalytics
	err := r.db.WithContext(ctx).
		Where("restaurant_id = ? AND date BETWEEN ? AND ?", restaurantID, start, end).
		Order("date ASC").
		Find(&stats).Error
	return stats, err
}

func (r *AnalyticsRepository) GetRestaurantByPartnerID(ctx context.Context, partnerID uuid.UUID) (*domain.Restaurant, error) {
	var restaurant domain.Restaurant
	err := r.db.WithContext(ctx).
		Where("partner_id = ? AND is_active = ?", partnerID, true).
		First(&restaurant).Error
	if err != nil {
		return nil, err
	}
	return &restaurant, nil
}
