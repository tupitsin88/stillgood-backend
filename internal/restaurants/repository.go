package restaurants

import (
	"fmt"
	"kursach_backend/internal/domain"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type OfferMeta struct {
	Categories      []string
	HasActiveOffers bool
}

type Repository interface {
	GetList(params ListParams) ([]domain.Restaurant, int64, error)
	GetByID(id string) (*domain.Restaurant, error)
	GetByPartnerID(partnerID uuid.UUID) (*domain.Restaurant, error)
	UpdatePartnerProfile(partnerID uuid.UUID, req PartnerRestaurantUpdateRequest) (*domain.Restaurant, error)
	GetOfferMetaByRestaurantIDs(restaurantIDs []uuid.UUID) (map[uuid.UUID]OfferMeta, error)
	IsPartner(userID uuid.UUID) (bool, error)
}

type repository struct {
	db *gorm.DB
}

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

	if params.Lat != nil && params.Lng != nil && params.Radius != nil {
		radiusKm := float64(*params.Radius) / 1000.0
		distanceSQL := fmt.Sprintf(
			"(6371 * acos(cos(radians(%f)) * cos(radians(restaurants.latitude)) * cos(radians(restaurants.longitude) - radians(%f)) + sin(radians(%f)) * sin(radians(restaurants.latitude))))",
			*params.Lat, *params.Lng, *params.Lat,
		)
		query = query.Where(distanceSQL+" <= ?", radiusKm).Order(distanceSQL + " ASC")
	} else {
		query = query.Order("restaurants.created_at DESC")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Limit(params.Limit).Offset(params.Offset).Find(&restaurants).Error; err != nil {
		return nil, 0, err
	}
	return restaurants, total, nil
}

func (r *repository) GetByID(id string) (*domain.Restaurant, error) {
	var restaurant domain.Restaurant
	if err := r.db.First(&restaurant, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &restaurant, nil
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
	if req.ImageURL != nil {
		updates["image_url"] = *req.ImageURL
	}

	if len(updates) > 0 {
		if err := r.db.Model(&domain.Restaurant{}).Where("partner_id = ?", partnerID).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	return r.GetByPartnerID(partnerID)
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

func (r *repository) IsPartner(userID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.Model(&domain.User{}).
		Where("id = ? AND role = ?", userID, "PARTNER").
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
