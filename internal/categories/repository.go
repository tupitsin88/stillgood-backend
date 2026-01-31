package categories

import (
	"kursach_backend/internal/domain"

	"gorm.io/gorm"
)

type Repository interface {
	GetAll() ([]domain.Category, error)
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) GetAll() ([]domain.Category, error) {
	var categories []domain.Category
	if err := r.db.Find(&categories).Error; err != nil {
		return nil, err
	}
	return categories, nil
}
