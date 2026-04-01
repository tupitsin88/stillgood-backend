package auth

import (
	"kursach_backend/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	CreateUser(user *domain.User) error
	GetUserByEmail(email string) (*domain.User, error)
	GetByID(id uuid.UUID) (*domain.User, error)
	UpdateDeviceToken(userID uuid.UUID, token string) error
	UpdatePasswordHash(userID uuid.UUID, passwordHash string) error
	ExistsByEmail(email string) (bool, error)
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) CreateUser(user *domain.User) error {
	return r.db.Create(user).Error
}

func (r *repository) GetUserByEmail(email string) (*domain.User, error) {
	var user domain.User
	err := r.db.Where("LOWER(email) = LOWER(?)", email).First(&user).Error
	return &user, err
}

func (r *repository) GetByID(id uuid.UUID) (*domain.User, error) {
	var user domain.User
	err := r.db.Where("id = ?", id).First(&user).Error
	return &user, err
}

func (r *repository) UpdateDeviceToken(userID uuid.UUID, token string) error {
	return r.db.Model(&domain.User{}).Where("id = ?", userID).Update("device_token", token).Error
}

func (r *repository) UpdatePasswordHash(userID uuid.UUID, passwordHash string) error {
	return r.db.Model(&domain.User{}).Where("id = ?", userID).Update("password_hash", passwordHash).Error
}

func (r *repository) ExistsByEmail(email string) (bool, error) {
	var count int64
	err := r.db.Model(&domain.User{}).Where("LOWER(email) = LOWER(?)", email).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
