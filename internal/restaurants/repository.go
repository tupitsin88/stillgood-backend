package restaurants

import (
	"fmt"
	"kursach_backend/internal/domain"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	GetList(params ListParams) ([]domain.Restaurant, int64, error)
	GetByID(id string) (*domain.Restaurant, error)
	GetByPartnerID(partnerID uuid.UUID) (*domain.Restaurant, error)
	UpdatePartnerProfile(partnerID uuid.UUID, req PartnerRestaurantUpdateRequest) (*domain.Restaurant, error)
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
	query = query.Distinct("restaurants.id")

	if params.CategoryID != nil {
		query = query.
			Joins("JOIN offers ON offers.restaurant_id = restaurants.id").
			Where("offers.category_id = ?", *params.CategoryID).
			Where("offers.is_active = ?", true).
			Where("offers.quantity_available > 0").
			Where("offers.pickup_time_end > ?", time.Now())
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
