package dto

import "github.com/google/uuid"

type RestaurantResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Address     string    `json:"address"`
	City        string    `json:"city"`
	Phone       string    `json:"phone"`
	Latitude    float64   `json:"latitude"`
	Longitude   float64   `json:"longitude"`
	ImageURL    string    `json:"image_url"`
	Rating      float64   `json:"rating"`
}
