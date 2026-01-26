package domain

import "time"

type User struct {
	ID            int       `gorm:"primaryKey" json:"id"`
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
