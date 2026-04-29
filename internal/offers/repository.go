package offers

import (
	"context"
	"kursach_backend/internal/domain"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OfferRepository struct {
	db *gorm.DB
}

const postgisPointSQL = "ST_SetSRID(ST_MakePoint(?, ?), 4326)::geography"
const restaurantDistanceSQL = "ST_Distance(restaurants.location, " + postgisPointSQL + ")"

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
		Where("restaurants.is_active = ?", true).
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

	hasGeoPoint := params.Lat != nil && params.Lng != nil
	if hasGeoPoint && params.Radius != nil {
		query = query.Where("ST_DWithin(restaurants.location, "+postgisPointSQL+", ?)", *params.Lng, *params.Lat, *params.Radius)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	switch params.SortBy {
	case "price":
		query = query.Order("offers.price ASC")
	case "pickupTime":
		query = query.Order("offers.pickup_time_start ASC")
	case "rating":
		query = query.Order("restaurants.rating DESC")
	case "distance":
		if hasGeoPoint {
			query = query.Order(distanceOrder(*params.Lng, *params.Lat))
		} else {
			query = query.Order("offers.id DESC")
		}
	default:
		query = query.Order("offers.pickup_time_start ASC")
	}
	if hasGeoPoint {
		query = query.Select("offers.*, ROUND("+restaurantDistanceSQL+")::int AS distance_meters", *params.Lng, *params.Lat)
	}

	err := query.
		Limit(params.Limit).
		Offset(params.Offset).
		Find(&offers).Error

	return offers, total, err
}

func distanceOrder(lng, lat float64) clause.OrderBy {
	return clause.OrderBy{
		Expression: clause.Expr{
			SQL:                restaurantDistanceSQL + " ASC",
			Vars:               []interface{}{lng, lat},
			WithoutParentheses: true,
		},
	}
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

func (r *OfferRepository) GetPartnerByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).First(&user, "id = ?", id).Error
	return &user, err
}
