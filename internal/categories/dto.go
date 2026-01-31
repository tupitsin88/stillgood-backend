package categories

import "github.com/google/uuid"

type CategoryResponse struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	IconURL *string   `json:"icon_url"`
}
