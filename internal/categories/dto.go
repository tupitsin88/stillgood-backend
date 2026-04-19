package categories

import "github.com/google/uuid"

type CategoryResponse struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	IconURL *string   `json:"icon_url"`
}

type CreateCategoryRequest struct {
	Name    string  `json:"name" binding:"required"`
	IconURL *string `json:"icon_url"`
}
