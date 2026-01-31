package restaurants

import (
	"kursach_backend/internal/domain"

	"gorm.io/gorm"
)

type Repository interface {
	GetAll() ([]domain.Restaurant, error)
	GetByID(id string) (*domain.Restaurant, error)
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) GetAll() ([]domain.Restaurant, error) {
	var restaurants []domain.Restaurant
	if err := r.db.Find(&restaurants).Error; err != nil {
		return nil, err
	}
	return restaurants, nil
}

func (r *repository) GetByID(id string) (*domain.Restaurant, error) {
	var restaurant domain.Restaurant
	if err := r.db.First(&restaurant, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &restaurant, nil
}
