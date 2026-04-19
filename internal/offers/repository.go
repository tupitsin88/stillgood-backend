package offers

import (
	"context"
	"fmt"
	"kursach_backend/internal/domain"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type OfferRepository struct {
	db *gorm.DB
}

func NewOfferRepository(db *gorm.DB) *OfferRepository {
	return &OfferRepository{db: db}
}

func (r *OfferRepository) Create(ctx context.Context, offer *domain.Offer) error {
	return r.db.WithContext(ctx).Create(offer).Error
}

func (r *OfferRepository) Update(ctx context.Context, offer *domain.Offer) error {
	return r.db.WithContext(ctx).Save(offer).Error
}

func (r *OfferRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Offer, error) {
	var offer domain.Offer
	err := r.db.WithContext(ctx).
		Preload("Restaurant").
		Preload("Category").
		First(&offer, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &offer, nil
}

func (r *OfferRepository) GetPartnerOffers(ctx context.Context, restaurantID uuid.UUID, limit, offset int) ([]domain.Offer, int64, error) {
	var offers []domain.Offer
	var total int64

	query := r.db.WithContext(ctx).
		Model(&domain.Offer{}).
		Where("restaurant_id = ?", restaurantID).
		Preload("Restaurant").
		Preload("Category")

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&offers).Error

	return offers, total, err
}

func (r *OfferRepository) GetPublicOffers(ctx context.Context, params FilterParams) ([]domain.Offer, int64, error) {
	var offers []domain.Offer
	var total int64

	query := r.db.WithContext(ctx).
		Model(&domain.Offer{}).
		Joins("JOIN restaurants ON restaurants.id = offers.restaurant_id").
		Preload("Restaurant").
		Preload("Category")
	query = query.
		Where("offers.quantity_available > 0").
		Where("offers.pickup_time_end > ?", time.Now())

	if params.IsActive != nil {
		query = query.Where("offers.is_active = ?", *params.IsActive)
	} else {
		query = query.Where("offers.is_active = ?", true)
	}
	if params.RestaurantID != nil {
		query = query.Where("offers.restaurant_id = ?", *params.RestaurantID)
	}
	if params.CategoryID != nil {
		query = query.Where("offers.category_id = ?", *params.CategoryID)
	}
	if params.MinPrice != nil {
		query = query.Where("offers.price >= ?", *params.MinPrice)
	}
	if params.MaxPrice != nil {
		query = query.Where("offers.price <= ?", *params.MaxPrice)
	}

	if params.Lat != nil && params.Lng != nil && params.Radius != nil {
		radiusKm := float64(*params.Radius) / 1000.0
		distanceSQL := fmt.Sprintf(
			"(6371 * acos(cos(radians(%f)) * cos(radians(restaurants.latitude) - radiade)) * cos(radians(restaurants.longituns(%f)) + sin(radians(%f)) * sin(radians(restaurants.latitude))))",
			*params.Lat, *params.Lng, *params.Lat,
		)
		query = query.Where(distanceSQL+" <= ?", radiusKm)
	}

	switch params.SortBy {
	case "price":
		query = query.Order("offers.price ASC")
	case "pickupTime":
		query = query.Order("offers.pickup_time_start ASC")
	case "rating":
		query = query.Order("restaurants.rating DESC")
	case "distance":
		if params.Lat != nil && params.Lng != nil {
			distanceSQL := fmt.Sprintf(
				"(6371 * acos(cos(radians(%f)) * cos(radians(restaurants.latitude)) * cos(radians(restaurants.longitude) - radians(%f)) + sin(radians(%f)) * sin(radians(restaurants.latitude))))",
				*params.Lat, *params.Lng, *params.Lat,
			)
			query = query.Order(distanceSQL + " ASC")
		} else {
			query = query.Order("offers.id DESC")
		}
	default:
		query = query.Order("offers.pickup_time_start ASC")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.
		Limit(params.Limit).
		Offset(params.Offset).
		Find(&offers).Error

	return offers, total, err
}

func (r *OfferRepository) GetRestaurantByPartnerID(ctx context.Context, partnerID uuid.UUID) (*domain.Restaurant, error) {
	var restaurant domain.Restaurant
	err := r.db.WithContext(ctx).
		Where("partner_id = ?", partnerID).
		First(&restaurant).Error
	return &restaurant, err
}

func (r *OfferRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&domain.Offer{}, "id = ?", id).Error
}
