package domain

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID            uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Email         string     `json:"email"`
	Role          string     `json:"role"`
	RestaurantID  *uuid.UUID `gorm:"type:uuid" json:"restaurant_id,omitempty"`
	PartnerStatus string     `json:"partner_status"`
	IsBlocked     bool       `gorm:"not null;default:false" json:"is_blocked"`
	DeviceToken   *string    `json:"device_token"`
	AuthProvider  string     `json:"auth_provider"`
	PasswordHash  string     `json:"-"`
	Name          string     `json:"name"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
