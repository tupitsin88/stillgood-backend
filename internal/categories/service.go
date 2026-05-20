package categories

import (
	"errors"
	"kursach_backend/internal/domain"

	"github.com/google/uuid"
)

var ErrCategoryHasOffers = errors.New("category has linked offers")

type Service interface {
	GetAll() ([]domain.Category, error)
	Create(name string, iconURL *string) (*domain.Category, error)
	Update(id uuid.UUID, name string, iconURL *string) (*domain.Category, error)
	Delete(id uuid.UUID) error
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) GetAll() ([]domain.Category, error) {
	return s.repo.GetAll()
}

func (s *service) Create(name string, iconURL *string) (*domain.Category, error) {
	category := &domain.Category{
		ID:      uuid.New(),
		Name:    name,
		IconURL: iconURL,
	}
	return category, s.repo.Create(category)
}

func (s *service) Update(id uuid.UUID, name string, iconURL *string) (*domain.Category, error) {
	category, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	category.Name = name
	category.IconURL = iconURL
	return category, s.repo.Update(category)
}

func (s *service) Delete(id uuid.UUID) error {
	hasActiveOffers, err := s.repo.HasActiveOffers(id)
	if err != nil {
		return err
	}
	if hasActiveOffers {
		return ErrCategoryHasOffers
	}
	return s.repo.Delete(id)
}
