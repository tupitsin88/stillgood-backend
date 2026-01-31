package domain

import (
	"github.com/google/uuid"
)

type Restaurant struct {
	ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Address     string    `json:"address"`
	City        string    `json:"city"`
	Phone       string    `json:"phone"`
	Latitude    float64   `json:"latitude"`
	Longitude   float64   `json:"longitude"`
	ImageURL    string    `json:"image_url"`
	Rating      float64   `gorm:"default:0" json:"rating"`
}
