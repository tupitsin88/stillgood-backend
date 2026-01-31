package restaurants

import (
	"mime/multipart"

	"kursach_backend/internal/domain"
	"kursach_backend/internal/pkg/filestorage"
)

type Service interface {
	GetAll() ([]domain.Restaurant, error)
	GetByID(id string) (*domain.Restaurant, error)
	UploadImage(file *multipart.FileHeader) (string, error)
}

type service struct {
	repo        Repository
	fileStorage *filestorage.FileStorage
}

func NewService(repo Repository, fileStorage *filestorage.FileStorage) Service {
	return &service{
		repo:        repo,
		fileStorage: fileStorage,
	}
}

func (s *service) GetAll() ([]domain.Restaurant, error) {
	return s.repo.GetAll()
}

func (s *service) GetByID(id string) (*domain.Restaurant, error) {
	return s.repo.GetByID(id)
}

func (s *service) UploadImage(file *multipart.FileHeader) (string, error) {
	return s.fileStorage.Upload(file)
}
