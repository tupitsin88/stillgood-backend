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
		WITH daily_orders AS (
			SELECT off.restaurant_id, off.category_id, COUNT(o.id) as bookings
			FROM orders o
			JOIN offers off ON o.offer_id = off.id
			WHERE o.created_at::date = ?::date AND o.status != 'CREATED'
			GROUP BY 1, 2
		),
		status_changes AS (
			SELECT 
				off.restaurant_id, 
				off.category_id,
				COUNT(h.id) FILTER (WHERE h.status = 'COMPLETED') as completed,
				COUNT(h.id) FILTER (WHERE h.status = 'CANCELLED') as cancelled,
				COUNT(h.id) FILTER (WHERE h.status = 'CANCELLED' AND o.cancellation_reason = 'expired') as expired,
				COALESCE(SUM(o.amount) FILTER (WHERE h.status = 'COMPLETED'), 0) as revenue
			FROM order_status_histories h
			JOIN orders o ON h.order_id = o.id
			JOIN offers off ON o.offer_id = off.id
			WHERE h.changed_at::date = ?::date
			GROUP BY 1, 2
		)	
		SELECT 
			COALESCE(d.restaurant_id, s.restaurant_id) as restaurant_id,
			c.name as category_name,
			COALESCE(d.bookings, 0) as total_bookings,
			COALESCE(s.completed, 0) as completed_orders,
			COALESCE(s.cancelled, 0) as cancelled_orders,
			COALESCE(s.expired, 0) as expired_orders,
			COALESCE(s.revenue, 0) as gross_revenue,
			res.commission as restaurant_commission -- ТЯНЕМ КОМИССИЮ ИЗ БД
		FROM daily_orders d
		FULL OUTER JOIN status_changes s ON d.restaurant_id = s.restaurant_id AND d.category_id = s.category_id
		JOIN categories c ON c.id = COALESCE(d.category_id, s.category_id)
		JOIN restaurants res ON res.id = COALESCE(d.restaurant_id, s.restaurant_id)
	`
	type aggResult struct {
		RestaurantID         uuid.UUID
		CategoryName         string
		TotalBookings        int
		CompletedOrders      int
		CancelledOrders      int
		ExpiredOrders        int
		GrossRevenue         float64
		RestaurantCommission float64
	}
	var rawStats []aggResult
	err := r.db.WithContext(ctx).Raw(query, dateStr, dateStr).Scan(&rawStats).Error
	if err != nil {
		return nil, err
	}

	for _, s := range rawStats {
		commRate := s.RestaurantCommission / 100.0
		if commRate == 0 {
			commRate = 0.15
		}

		fee := s.GrossRevenue * commRate
		results = append(results, domain.DailyAnalytics{
			ID:              uuid.New(),
			RestaurantID:    s.RestaurantID,
			Date:            date,
			CategoryName:    s.CategoryName,
			TotalBookings:   s.TotalBookings,
			CompletedOrders: s.CompletedOrders,
			CancelledOrders: s.CancelledOrders,
			ExpiredOrders:   s.ExpiredOrders,
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
			"total_bookings", "completed_orders", "cancelled_orders", "expired_orders",
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
