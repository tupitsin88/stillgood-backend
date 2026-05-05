package domain

import (
	"time"

	"github.com/google/uuid"
)

type RefreshSession struct {
	JTI       uuid.UUID  `gorm:"column:jti;type:uuid;primaryKey" json:"jti"`
	UserID    uuid.UUID  `gorm:"type:uuid;index" json:"user_id"`
	ExpiresAt time.Time  `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

func (RefreshSession) TableName() string {
	return "refresh_sessions"
}
