package domain

import "time"
import "github.com/google/uuid"

type Offer struct {
	ID                uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	RestaurantID      uuid.UUID  `gorm:"type:uuid" json:"restaurant_id"`
	Restaurant        Restaurant `gorm:"foreignKey:RestaurantID" json:"restaurant"`
	Title             string     `json:"title"`
	Description       string     `json:"description"`
	CategoryID        uuid.UUID  `gorm:"type:uuid" json:"category_id"`
	Category          Category   `gorm:"foreignKey:CategoryID" json:"category"`
	ImageURL          *string    `json:"image_url"`
	Price             float64    `json:"price"`
	OriginalPrice     float64    `json:"original_price"`
	QuantityAvailable int        `json:"quantity_available"`
	QuantityTotal     int        `json:"quantity_total"`
	PickupStart       time.Time  `json:"pickup_time_start" gorm:"column:pickup_time_start"`
	PickupEnd         time.Time  `json:"pickup_time_end" gorm:"column:pickup_time_end"`
	CreatedAt         time.Time  `json:"created_at"`
	IsActive          bool       `json:"is_active"`

	DistanceMeters *int `gorm:"column:distance_meters;->;-:migration" json:"-"`
}
