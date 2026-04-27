package notifications

import (
	"context"
	"errors"
	"kursach_backend/internal/domain"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Store interface {
	Create(ctx context.Context, notification *domain.Notification) error
	GetDeviceToken(ctx context.Context, userID uuid.UUID) (string, bool, error)
	ListForUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.Notification, error)
	CleanupOld(ctx context.Context, olderThan time.Time) error
}

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, notification *domain.Notification) error {
	return r.db.WithContext(ctx).Create(notification).Error
}

func (r *Repository) GetDeviceToken(ctx context.Context, userID uuid.UUID) (string, bool, error) {
	var user domain.User
	err := r.db.WithContext(ctx).
		Select("device_token").
		First(&user, "id = ?", userID).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if user.DeviceToken == nil {
		return "", false, nil
	}

	token := strings.TrimSpace(*user.DeviceToken)
	if token == "" {
		return "", false, nil
	}
	return token, true, nil
}

func (r *Repository) ListForUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.Notification, error) {
	var notifications []domain.Notification
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&notifications).Error
	return notifications, err
}

func (r *Repository) CleanupOld(ctx context.Context, olderThan time.Time) error {
	return r.db.WithContext(ctx).Where("created_at < ?", olderThan).Delete(&domain.Notification{}).Error
}
