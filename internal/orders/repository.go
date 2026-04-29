package orders

import (
	"context"
	"kursach_backend/internal/domain"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OrderRepository struct {
	db *gorm.DB
}

func NewOrderRepository(db *gorm.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

func (r *OrderRepository) Transaction(fn func(txRepo *OrderRepository) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		txRepo := &OrderRepository{db: tx}
		return fn(txRepo)
	})
}

func (r *OrderRepository) SaveHistory(ctx context.Context, history domain.OrderStatusHistory) error {
	return r.db.WithContext(ctx).Create(&history).Error
}

func (r *OrderRepository) CreateOrder(ctx context.Context, order *domain.Order) error {
	return r.db.WithContext(ctx).Create(order).Error
}

func (r *OrderRepository) GetByIDWithDetails(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	var order domain.Order
	err := r.db.WithContext(ctx).
		Preload("User").
		Preload("Offer").
		Preload("Offer.Restaurant").
		First(&order, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *OrderRepository) GetExpiredOrders(ctx context.Context) ([]domain.Order, error) {
	var orders []domain.Order
	err := r.db.WithContext(ctx).
		Where("status = ? AND expires_at < ?", domain.OrderCreated, time.Now()).
		Find(&orders).Error
	return orders, err
}

func (r *OrderRepository) GetUserOrders(ctx context.Context, userID uuid.UUID, limit, offset int, statusFilter []string) ([]domain.Order, int64, error) {
	var orders []domain.Order
	var total int64

	query := r.db.WithContext(ctx).
		Model(&domain.Order{}).
		Where("user_id = ?", userID).
		Preload("Offer").
		Preload("Offer.Restaurant")

	if len(statusFilter) > 0 {
		query = query.Where("status IN ?", statusFilter)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&orders).Error

	return orders, total, err
}

func (r *OrderRepository) GetUserStats(ctx context.Context, userID uuid.UUID) (int, float64, error) {
	var stats struct {
		TotalBoxes int
		TotalSaved float64
	}
	err := r.db.WithContext(ctx).
		Table("orders o").
		Select("COUNT(o.id) as total_boxes, SUM(off.original_price - o.amount) as total_saved").
		Joins("JOIN offers off ON o.offer_id = off.id").
		Where("o.user_id = ? AND o.status = ?", userID, "COMPLETED").
		Scan(&stats).Error
	return stats.TotalBoxes, stats.TotalSaved, err
}

func (r *OrderRepository) GetPartnerOrdersWithTotal(ctx context.Context, restaurantID uuid.UUID, limit, offset int, statuses []string) ([]domain.Order, int64, error) {
	var orders []domain.Order
	var total int64
	query := r.db.WithContext(ctx).
		Model(&domain.Order{}).
		Joins("JOIN offers ON orders.offer_id = offers.id").
		Where("offers.restaurant_id = ?", restaurantID)
	if len(statuses) > 0 {
		query = query.Where("orders.status IN ?", statuses)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.
		Preload("User").
		Preload("Offer").
		Order("orders.created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&orders).Error

	return orders, total, err
}

func (r *OrderRepository) Update(ctx context.Context, order *domain.Order) error {
	return r.db.WithContext(ctx).Save(order).Error
}

func (r *OrderRepository) GetOfferByIDForUpdate(ctx context.Context, id uuid.UUID) (*domain.Offer, error) {
	var offer domain.Offer
	err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Preload("Restaurant").
		First(&offer, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &offer, err
}

func (r *OrderRepository) UpdateOfferQuantity(ctx context.Context, offerID uuid.UUID, delta int) error {
	return r.db.WithContext(ctx).
		Model(&domain.Offer{}).
		Where("id = ?", offerID).
		Update("quantity_available", gorm.Expr("quantity_available + ?", delta)).Error
}

func (r *OrderRepository) CreateReview(ctx context.Context, review *domain.Review) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(review).Error; err != nil {
			return err
		}
		err := tx.Model(&domain.Restaurant{}).
			Where("id = ?", review.RestaurantID).
			Updates(map[string]interface{}{
				"rating": tx.Model(&domain.Review{}).
					Select("COALESCE(AVG(rating), 0)").
					Where("restaurant_id = ?", review.RestaurantID),
				"review_count": tx.Model(&domain.Review{}).
					Where("restaurant_id = ?", review.RestaurantID).
					Select("COUNT(*)"),
			}).Error
		return err
	})
}
