package domain

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID            uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Email         string    `json:"email"`
	Role          string    `json:"role"`
	PartnerStatus string    `json:"partner_status"`
	DeviceToken   *string   `json:"device_token"`
	AuthProvider  string    `json:"auth_provider"`
	PasswordHash  string    `json:"-"`
	Name          string    `json:"name"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
