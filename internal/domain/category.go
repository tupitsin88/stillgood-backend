package domain

import (
	"github.com/google/uuid"
)

type Category struct {
	ID      uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name    string    `json:"name"`
	IconURL *string   `json:"icon_url"`
}
