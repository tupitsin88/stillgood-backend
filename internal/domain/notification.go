package domain

import (
	"time"

	"github.com/google/uuid"
)

type Notification struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid;index" json:"user_id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	DeepLink  string    `json:"deepLink"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
}
