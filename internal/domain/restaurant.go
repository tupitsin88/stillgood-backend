package domain

import "time"

type Restaurant struct {
	ID           int       `gorm:"primaryKey" json:"id"`
	PartnerID    int       `json:"partner_id"`
	Name         string    `json:"name"`
	CompanyName  string    `json:"company_name"`
	Inn          string    `json:"inn"`
	Address      string    `json:"address"`
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
}
