package orders

import (
	"context"
	"kursach_backend/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type OrderRepository struct {
	db *gorm.DB
}

func NewOrderRepository(db *gorm.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

func (r *OrderRepository) CreateOrder(ctx context.Context, order *domain.Order) error {
	return r.db.WithContext(ctx).Create(order).Error
}

func (r *OrderRepository) GetByIDWithDetails(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	var order domain.Order
	err := r.db.WithContext(ctx).
		Preload("Offer").
		Preload("Offer.Restaurant").
		First(&order, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
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

func (r *OrderRepository) GetPartnerOrders(ctx context.Context, limit, offset int, statuses []string) ([]domain.Order, error) {
	var orders []domain.Order

	query := r.db.WithContext(ctx).
		Model(&domain.Order{}).
		Preload("User").
		Preload("Offer").
		Where("status IN ?", statuses)

	// TODO: Добавить фильтр по PartnerID (через Offer.Restaurant.PartnerID) когда будет авторизация партнера

	err := query.
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&orders).Error

	return orders, err
}

func (r *OrderRepository) Update(ctx context.Context, order *domain.Order) error {
	return r.db.WithContext(ctx).Save(order).Error
}

func (r *OrderRepository) GetOfferByID(ctx context.Context, offerID uuid.UUID) (*domain.Offer, error) {
	var offer domain.Offer
	err := r.db.WithContext(ctx).
		Preload("Restaurant").
		First(&offer, "id = ?", offerID).Error
	return &offer, err
}

func (r *OrderRepository) UpdateOfferQuantity(ctx context.Context, offerID uuid.UUID, delta int) error {
	return r.db.WithContext(ctx).
		Model(&domain.Offer{}).
		Where("id = ?", offerID).
		Update("quantity_available", gorm.Expr("quantity_available + ?", delta)).Error
}
