package categories

import "kursach_backend/internal/domain"

type Service interface {
	GetAll() ([]domain.Category, error)
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
