package domain

import "time"

// Offer соответствует таблице `offers` из BD.pdf
type Offer struct {
	ID           int        `gorm:"primaryKey" json:"id"`
	RestaurantID int        `json:"restaurant_id"`
	Restaurant   Restaurant `gorm:"foreignKey:RestaurantID" json:"restaurant"`

	Title             string    `json:"title"`
	Description       string    `json:"description"`
	CategoryID        int       `json:"category_id"`
	ImageURL          *string   `json:"image_url"`
	Price             float64   `json:"price"`
	OriginalPrice     float64   `json:"original_price"`
	QuantityAvailable int       `json:"quantity_available"`
	PickupStart       time.Time `json:"pickup_time_start" gorm:"column:pickup_time_start"` // В БД pickup_time_start
	PickupEnd         time.Time `json:"pickup_time_end" gorm:"column:pickup_time_end"`     // В БД pickup_time_end
	CreatedAt         time.Time `json:"created_at"`
	IsActive          bool      `json:"is_active"`
}
