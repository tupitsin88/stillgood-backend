package restaurants

import (
	"context"
	"kursach_backend/internal/domain"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OfferMeta struct {
	Categories      []string
	HasActiveOffers bool
}

type Repository interface {
	GetList(params ListParams) ([]domain.Restaurant, int64, error)
	GetAdminList(limit, offset int) ([]domain.Restaurant, int64, error)
	GetByID(id string) (*domain.Restaurant, error)
	CreateForPartner(restaurant *domain.Restaurant) error
	GetByPartnerID(partnerID uuid.UUID) (*domain.Restaurant, error)
	UpdatePartnerProfile(partnerID uuid.UUID, req PartnerRestaurantUpdateRequest) (*domain.Restaurant, error)
	UpdateAdminFields(id uuid.UUID, req AdminRestaurantUpdateRequest) (*domain.Restaurant, error)
	GetOfferMetaByRestaurantIDs(restaurantIDs []uuid.UUID) (map[uuid.UUID]OfferMeta, error)
	IsApprovedPartner(userID uuid.UUID) (bool, error)
	GetReviews(ctx context.Context, restID uuid.UUID, limit, offset int) ([]domain.Review, int64, error)
	GetAdminReviews(ctx context.Context, restaurantID *uuid.UUID, limit, offset int) ([]domain.Review, int64, error)
	DeleteReview(ctx context.Context, reviewID uuid.UUID) error
}

type repository struct {
	db *gorm.DB
}

const postgisPointSQL = "ST_SetSRID(ST_MakePoint(?, ?), 4326)::geography"
const restaurantDistanceSQL = "ST_Distance(restaurants.location, " + postgisPointSQL + ")"

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) GetList(params ListParams) ([]domain.Restaurant, int64, error) {
	var restaurants []domain.Restaurant
	var total int64

	query := r.db.Model(&domain.Restaurant{}).Where("restaurants.is_active = ?", true)

	if params.CategoryID != nil {
		query = query.Where(`
			EXISTS (
				SELECT 1
				FROM offers
				WHERE offers.restaurant_id = restaurants.id
					AND offers.category_id = ?
					AND offers.is_active = ?
					AND offers.quantity_available > 0
					AND offers.pickup_time_end > ?
			)
		`, *params.CategoryID, true, time.Now())
	}

	hasGeoPoint := params.Lat != nil && params.Lng != nil
	hasGeoFilter := hasGeoPoint && params.Radius != nil
	if hasGeoFilter {
		query = query.Where("ST_DWithin(restaurants.location, "+postgisPointSQL+", ?)", *params.Lng, *params.Lat, *params.Radius)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if hasGeoFilter {
		query = query.Order(distanceOrder(*params.Lng, *params.Lat))
	} else {
		query = query.Order("restaurants.created_at DESC")
	}
	if hasGeoPoint {
		query = query.Select("restaurants.*, ROUND("+restaurantDistanceSQL+")::int AS distance_meters", *params.Lng, *params.Lat)
	}

	if err := query.Limit(params.Limit).Offset(params.Offset).Find(&restaurants).Error; err != nil {
		return nil, 0, err
	}
	return restaurants, total, nil
}

func (r *repository) GetAdminList(limit, offset int) ([]domain.Restaurant, int64, error) {
	var restaurants []domain.Restaurant
	var total int64

	query := r.db.Model(&domain.Restaurant{})
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&restaurants).Error; err != nil {
		return nil, 0, err
	}
	return restaurants, total, nil
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

func (r *repository) GetByID(id string) (*domain.Restaurant, error) {
	var restaurant domain.Restaurant
	if err := r.db.First(&restaurant, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &restaurant, nil
}

func (r *repository) CreateForPartner(restaurant *domain.Restaurant) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(restaurant).Error; err != nil {
			return err
		}

		result := tx.Model(&domain.User{}).
			Where("id = ? AND role = ? AND partner_status = ? AND deleted_at IS NULL", restaurant.PartnerID, "PARTNER", "APPROVED").
			Update("restaurant_id", restaurant.ID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		return nil
	})
}

func (r *repository) GetByPartnerID(partnerID uuid.UUID) (*domain.Restaurant, error) {
	var restaurant domain.Restaurant
	if err := r.db.Where("partner_id = ?", partnerID).First(&restaurant).Error; err != nil {
		return nil, err
	}
	return &restaurant, nil
}

func (r *repository) UpdatePartnerProfile(partnerID uuid.UUID, req PartnerRestaurantUpdateRequest) (*domain.Restaurant, error) {
	updates := map[string]interface{}{}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.WorkingHours != nil {
		updates["working_hours"] = *req.WorkingHours
	}
	if req.LogoURL != nil {
		updates["logo_url"] = *req.LogoURL
	}
	if req.CoverURL != nil {
		updates["cover_url"] = *req.CoverURL
	}

	if len(updates) > 0 {
		if err := r.db.Model(&domain.Restaurant{}).Where("partner_id = ?", partnerID).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	return r.GetByPartnerID(partnerID)
}

func (r *repository) UpdateAdminFields(id uuid.UUID, req AdminRestaurantUpdateRequest) (*domain.Restaurant, error) {
	updates := map[string]interface{}{}
	if req.Commission != nil {
		updates["commission"] = *req.Commission
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if len(updates) > 0 {
		result := r.db.Model(&domain.Restaurant{}).Where("id = ?", id).Updates(updates)
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected == 0 {
			return nil, gorm.ErrRecordNotFound
		}
	}

	var restaurant domain.Restaurant
	if err := r.db.First(&restaurant, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &restaurant, nil
}

func (r *repository) GetOfferMetaByRestaurantIDs(restaurantIDs []uuid.UUID) (map[uuid.UUID]OfferMeta, error) {
	metaByID := make(map[uuid.UUID]OfferMeta)
	if len(restaurantIDs) == 0 {
		return metaByID, nil
	}

	type row struct {
		RestaurantID uuid.UUID
		CategoryName string
	}

	var rows []row
	err := r.db.
		Table("offers o").
		Select("o.restaurant_id AS restaurant_id, c.name AS category_name").
		Joins("JOIN categories c ON c.id = o.category_id").
		Where("o.restaurant_id IN ?", restaurantIDs).
		Where("o.is_active = ?", true).
		Where("o.quantity_available > 0").
		Where("o.pickup_time_end > ?", time.Now()).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	seenCategory := make(map[uuid.UUID]map[string]struct{})
	for _, r := range rows {
		meta := metaByID[r.RestaurantID]
		meta.HasActiveOffers = true

		if _, ok := seenCategory[r.RestaurantID]; !ok {
			seenCategory[r.RestaurantID] = make(map[string]struct{})
		}
		if _, exists := seenCategory[r.RestaurantID][r.CategoryName]; !exists {
			seenCategory[r.RestaurantID][r.CategoryName] = struct{}{}
			meta.Categories = append(meta.Categories, r.CategoryName)
		}

		metaByID[r.RestaurantID] = meta
	}

	return metaByID, nil
}

func (r *repository) IsApprovedPartner(userID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.Model(&domain.User{}).
		Where("id = ? AND role = ? AND partner_status = ?", userID, "PARTNER", "APPROVED").
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *repository) GetReviews(ctx context.Context, restID uuid.UUID, limit, offset int) ([]domain.Review, int64, error) {
	return r.GetAdminReviews(ctx, &restID, limit, offset)
}

func (r *repository) GetAdminReviews(ctx context.Context, restaurantID *uuid.UUID, limit, offset int) ([]domain.Review, int64, error) {
	var reviews []domain.Review
	var total int64
	db := r.db.WithContext(ctx).Model(&domain.Review{})
	if restaurantID != nil {
		db = db.Where("restaurant_id = ?", *restaurantID)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := db.Preload("User").Order("created_at DESC").Limit(limit).Offset(offset).Find(&reviews).Error
	return reviews, total, err
}

func (r *repository) DeleteReview(ctx context.Context, reviewID uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var review domain.Review
		if err := tx.First(&review, "id = ?", reviewID).Error; err != nil {
			return err
		}

		if err := tx.Delete(&review).Error; err != nil {
			return err
		}

		return tx.Model(&domain.Restaurant{}).
			Where("id = ?", review.RestaurantID).
			Updates(map[string]interface{}{
				"rating": tx.Model(&domain.Review{}).
					Select("COALESCE(AVG(rating), 0)").
					Where("restaurant_id = ?", review.RestaurantID),
				"review_count": tx.Model(&domain.Review{}).
					Select("COUNT(*)").
					Where("restaurant_id = ?", review.RestaurantID),
			}).Error
	})
}
