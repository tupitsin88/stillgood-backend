package domain

import (
	"time"

	"github.com/google/uuid"
)

type Restaurant struct {
	ID           uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	PartnerID    uuid.UUID `gorm:"type:uuid" json:"partner_id"`
	Name         string    `json:"name"`
	CompanyName  string    `json:"company_name"`
	Inn          string    `json:"inn"`
	Address      string    `json:"address"`
	Description  *string   `json:"description,omitempty"`
	Commission   float64   `gorm:"not null;default:0" json:"commission"`
	Rating       float64   `json:"rating"`
	ReviewCount  int       `json:"review_count"`
	ImageURL     *string   `json:"image_url"`
	Phone        *string   `json:"phone"`
	Latitude     float64   `json:"latitude"`
	Longitude    float64   `json:"longitude"`
	IsActive     bool      `json:"is_active"`
	WorkingHours string    `json:"working_hours"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	DistanceMeters *int `gorm:"column:distance_meters;->;-:migration" json:"-"`
}
