package categories

import (
	"kursach_backend/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	GetAll() ([]domain.Category, error)
	GetByID(id uuid.UUID) (*domain.Category, error)
	Create(category *domain.Category) error
	Update(category *domain.Category) error
	Delete(id uuid.UUID) error
	HasActiveOffers(categoryID uuid.UUID) (bool, error)
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

func (r *repository) Create(category *domain.Category) error {
	return r.db.Create(category).Error
}

func (r *repository) Update(category *domain.Category) error {
	return r.db.Save(category).Error
}

func (r *repository) Delete(id uuid.UUID) error {
	return r.db.Delete(&domain.Category{}, "id = ?", id).Error
}

func (r *repository) GetByID(id uuid.UUID) (*domain.Category, error) {
	var category domain.Category
	err := r.db.First(&category, "id = ?", id).Error
	return &category, err
}

func (r *repository) HasActiveOffers(categoryID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.Table("offers").Where("category_id = ?", categoryID).Count(&count).Error
	return count > 0, err
}
