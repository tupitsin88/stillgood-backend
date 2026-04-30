package auth

import (
	"strings"
	"time"

	"kursach_backend/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	CreateUser(user *domain.User) error
	GetUserByEmail(email string) (*domain.User, error)
	GetByID(id uuid.UUID) (*domain.User, error)
	IsUserBlocked(id uuid.UUID) (bool, error)
	ListPartnersByStatus(status string, limit, offset int) ([]domain.User, int64, error)
	ListUsersByRoles(roles []string, limit, offset int, search string) ([]domain.User, int64, error)
	UpdatePartnerStatus(userID uuid.UUID, status string) error
	UpdateBlockedStatus(userID uuid.UUID, isBlocked bool) error
	UpdateDeviceToken(userID uuid.UUID, token string) error
	UpdatePasswordHash(userID uuid.UUID, passwordHash string) error
	UpdateName(userID uuid.UUID, name string) error
	UpdatePhone(userID uuid.UUID, phone *string) error
	UpdateVerifiedStatusByEmail(email string, isVerified bool) error
	UpdateEmailAndResetVerification(userID uuid.UUID, email string) error
	CountActiveOrdersByUserID(userID uuid.UUID) (int64, error)
	AnonymizeAccount(userID uuid.UUID) error
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

func (r *repository) IsUserBlocked(id uuid.UUID) (bool, error) {
	var result struct {
		IsBlocked bool
		DeletedAt *time.Time
	}
	if err := r.db.Model(&domain.User{}).Select("is_blocked", "deleted_at").Where("id = ?", id).First(&result).Error; err != nil {
		return false, err
	}
	return result.IsBlocked || result.DeletedAt != nil, nil
}

func (r *repository) ListPartnersByStatus(status string, limit, offset int) ([]domain.User, int64, error) {
	var (
		users []domain.User
		total int64
	)

	query := r.db.Model(&domain.User{}).Where("role = ? AND partner_status = ? AND deleted_at IS NULL", "PARTNER", status)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("created_at ASC").Limit(limit).Offset(offset).Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func (r *repository) ListUsersByRoles(roles []string, limit, offset int, search string) ([]domain.User, int64, error) {
	var (
		users []domain.User
		total int64
	)

	query := r.db.Model(&domain.User{}).Where("role IN ?", roles)
	if pattern := userSearchPattern(search); pattern != "" {
		query = query.Where(
			`email ILIKE ? ESCAPE '\' OR name ILIKE ? ESCAPE '\' OR phone ILIKE ? ESCAPE '\' OR id::text ILIKE ? ESCAPE '\'`,
			pattern,
			pattern,
			pattern,
			pattern,
		)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("created_at ASC").Limit(limit).Offset(offset).Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func userSearchPattern(search string) string {
	search = strings.TrimSpace(search)
	if search == "" {
		return ""
	}
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + replacer.Replace(search) + "%"
}

func (r *repository) UpdatePartnerStatus(userID uuid.UUID, status string) error {
	result := r.db.Model(&domain.User{}).
		Where("id = ? AND role = ? AND deleted_at IS NULL", userID, "PARTNER").
		Update("partner_status", status)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

func (r *repository) UpdateBlockedStatus(userID uuid.UUID, isBlocked bool) error {
	result := r.db.Model(&domain.User{}).Where("id = ?", userID).Update("is_blocked", isBlocked)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		var count int64
		if err := r.db.Model(&domain.User{}).Where("id = ?", userID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return gorm.ErrRecordNotFound
		}
	}
	return nil
}

func (r *repository) UpdateDeviceToken(userID uuid.UUID, token string) error {
	return r.db.Model(&domain.User{}).Where("id = ?", userID).Update("device_token", token).Error
}

func (r *repository) UpdatePasswordHash(userID uuid.UUID, passwordHash string) error {
	return r.db.Model(&domain.User{}).Where("id = ?", userID).Update("password_hash", passwordHash).Error
}

func (r *repository) UpdateName(userID uuid.UUID, name string) error {
	result := r.db.Model(&domain.User{}).Where("id = ?", userID).Update("name", name)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *repository) UpdatePhone(userID uuid.UUID, phone *string) error {
	result := r.db.Model(&domain.User{}).Where("id = ?", userID).Update("phone", phone)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *repository) UpdateVerifiedStatusByEmail(email string, isVerified bool) error {
	result := r.db.Model(&domain.User{}).
		Where("LOWER(email) = LOWER(?)", email).
		Update("is_verified", isVerified)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *repository) UpdateEmailAndResetVerification(userID uuid.UUID, email string) error {
	result := r.db.Model(&domain.User{}).
		Where("id = ?", userID).
		Updates(map[string]interface{}{
			"email":       email,
			"is_verified": false,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *repository) CountActiveOrdersByUserID(userID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&domain.Order{}).
		Where("user_id = ? AND status IN ?", userID, []domain.OrderStatus{domain.OrderCreated, domain.OrderPaid}).
		Count(&count).Error
	return count, err
}

func (r *repository) AnonymizeAccount(userID uuid.UUID) error {
	now := time.Now().UTC()
	result := r.db.Model(&domain.User{}).
		Where("id = ? AND deleted_at IS NULL", userID).
		Updates(map[string]interface{}{
			"email":          "deleted+" + userID.String() + "@deleted.local",
			"phone":          nil,
			"partner_status": "",
			"is_verified":    false,
			"is_blocked":     true,
			"device_token":   nil,
			"password_hash":  "",
			"name":           "Deleted User",
			"deleted_at":     &now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

func (r *repository) ExistsByEmail(email string) (bool, error) {
	var count int64
	err := r.db.Model(&domain.User{}).Where("LOWER(email) = LOWER(?)", email).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
