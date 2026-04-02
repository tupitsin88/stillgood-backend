package restaurants

import (
	"errors"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"kursach_backend/internal/domain"
	"kursach_backend/internal/pkg/filestorage"
)

type Service interface {
	GetList(params ListParams) ([]domain.Restaurant, int64, error)
	GetByID(id string) (*domain.Restaurant, error)
	GetPartnerRestaurant(partnerID string) (*domain.Restaurant, error)
	UpdatePartnerRestaurant(partnerID string, req PartnerRestaurantUpdateRequest) (*domain.Restaurant, error)
	UploadImage(file *multipart.FileHeader) (string, error)
}

type ListParams struct {
	Lat        *float64
	Lng        *float64
	Radius     *int
	CategoryID *uuid.UUID
	Limit      int
	Offset     int
}

var ErrInvalidImageFormat = errors.New("invalid image format")

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

func (s *service) GetList(params ListParams) ([]domain.Restaurant, int64, error) {
	return s.repo.GetList(params)
}

func (s *service) GetByID(id string) (*domain.Restaurant, error) {
	return s.repo.GetByID(id)
}

func (s *service) GetPartnerRestaurant(partnerID string) (*domain.Restaurant, error) {
	uid, err := uuid.Parse(partnerID)
	if err != nil {
		return nil, err
	}
	return s.repo.GetByPartnerID(uid)
}

func (s *service) UpdatePartnerRestaurant(partnerID string, req PartnerRestaurantUpdateRequest) (*domain.Restaurant, error) {
	uid, err := uuid.Parse(partnerID)
	if err != nil {
		return nil, err
	}
	return s.repo.UpdatePartnerProfile(uid, req)
}

func (s *service) UploadImage(file *multipart.FileHeader) (string, error) {
	ext := strings.ToLower(filepath.Ext(file.Filename))
	contentType := strings.ToLower(file.Header.Get("Content-Type"))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".heic" {
		return "", ErrInvalidImageFormat
	}
	if !strings.HasPrefix(contentType, "image/") {
		return "", ErrInvalidImageFormat
	}
	return s.fileStorage.Upload(file)
}
