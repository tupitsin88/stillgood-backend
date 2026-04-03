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
	CountActiveOrdersByUserID(userID uuid.UUID) (int64, error)
	DeleteAccount(userID uuid.UUID) error
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

func (r *repository) CountActiveOrdersByUserID(userID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&domain.Order{}).
		Where("user_id = ? AND status IN ?", userID, []domain.OrderStatus{domain.OrderCreated, domain.OrderPaid}).
		Count(&count).Error
	return count, err
}

func (r *repository) DeleteAccount(userID uuid.UUID) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		sub := tx.Model(&domain.Order{}).Select("id").Where("user_id = ?", userID)

		if err := tx.Where("order_id IN (?)", sub).Delete(&domain.OrderStatusHistory{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&domain.Order{}).Error; err != nil {
			return err
		}
		if err := tx.Where("id = ?", userID).Delete(&domain.User{}).Error; err != nil {
			return err
		}

		return nil
	})
}

func (r *repository) ExistsByEmail(email string) (bool, error) {
	var count int64
	err := r.db.Model(&domain.User{}).Where("LOWER(email) = LOWER(?)", email).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
