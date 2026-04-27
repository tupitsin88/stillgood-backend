package domain

import (
	"time"

	"github.com/google/uuid"
)

type Review struct {
	ID           uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	OrderID      uuid.UUID `gorm:"type:uuid;uniqueIndex" json:"order_id"`
	RestaurantID uuid.UUID `gorm:"type:uuid;index" json:"restaurant_id"`
	UserID       uuid.UUID `gorm:"type:uuid;index" json:"user_id"`
	Rating       int       `gorm:"not null;check:rating >= 1 AND rating <= 5" json:"rating"`
	Comment      string    `gorm:"type:varchar(500)" json:"comment"`
	CreatedAt    time.Time `json:"created_at"`
}
