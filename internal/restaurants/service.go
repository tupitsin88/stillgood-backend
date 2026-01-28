package restaurants

import (
	"kursach_backend/internal/domain"
)

type Service interface {
	GetAll() ([]domain.Restaurant, error)
	GetByID(id string) (*domain.Restaurant, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) GetAll() ([]domain.Restaurant, error) {
	return s.repo.GetAll()
}

func (s *service) GetByID(id string) (*domain.Restaurant, error) {
	return s.repo.GetByID(id)
}
