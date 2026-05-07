package offers

import (
	"time"

	"github.com/google/uuid"
)

type CreateOfferRequest struct {
	Title         string    `json:"title" binding:"required"`
	Description   string    `json:"description"`
	Price         float64   `json:"price" binding:"required,gt=0"`
	OriginalPrice float64   `json:"originalPrice" binding:"required,gt=0"`
	Quantity      int       `json:"quantity" binding:"required,min=1"`
	PickupStart   time.Time `json:"pickupStart" binding:"required"`
	PickupEnd     time.Time `json:"pickupEnd" binding:"required"`
	CategoryID    string    `json:"categoryId" binding:"required"`
	ImageURL      string    `json:"imageUrl"`
}

type UpdateOfferRequest struct {
	Title         *string    `json:"title"`
	Description   *string    `json:"description"`
	Price         *float64   `json:"price"`
	OriginalPrice *float64   `json:"originalPrice"`
	Quantity      *int       `json:"quantity"`
	IsActive      *bool      `json:"isActive"`
	PickupStart   *time.Time `json:"pickupStart"`
	PickupEnd     *time.Time `json:"pickupEnd"`
	CategoryID    *string    `json:"categoryId"`
	ImageURL      *string    `json:"imageUrl"`
}

type OfferDetailDTO struct {
	ID                string             `json:"id"`
	Title             string             `json:"title"`
	Description       string             `json:"description,omitempty"`
	Price             float64            `json:"price"`
	OriginalPrice     float64            `json:"originalPrice"`
	Discount          int                `json:"discount"`
	ImageURL          *string            `json:"imageUrl,omitempty"`
	QuantityAvailable int                `json:"quantityAvailable"`
	QuantityTotal     int                `json:"quantityTotal"`
	PickupStart       time.Time          `json:"pickupStart"`
	PickupEnd         time.Time          `json:"pickupEnd"`
	IsActive          bool               `json:"isActive"`
	Category          CategoryDTO        `json:"category"`
	Restaurant        RestaurantShortDTO `json:"restaurant"`
}

type OfferPreviewDTO struct {
	ID                string             `json:"id"`
	Title             string             `json:"title"`
	Price             float64            `json:"price"`
	OriginalPrice     float64            `json:"originalPrice"`
	Discount          int                `json:"discount"`
	ImageURL          *string            `json:"imageUrl,omitempty"`
	Distance          *int               `json:"distance,omitempty"`
	PickupStart       time.Time          `json:"pickupStart"`
	PickupEnd         time.Time          `json:"pickupEnd"`
	QuantityAvailable int                `json:"quantityAvailable"`
	Category          CategoryDTO        `json:"category"`
	Restaurant        RestaurantShortDTO `json:"restaurant"`
}

type CategoryDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type RestaurantShortDTO struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Address   string  `json:"address"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Phone     *string `json:"phone,omitempty"`
}

type FilterParams struct {
	Lat          *float64
	Lng          *float64
	Radius       *int
	RestaurantID *uuid.UUID
	CategoryID   *uuid.UUID
	MinPrice     *float64
	MaxPrice     *float64
	IsActive     *bool
	SortBy       string
	Limit        int
	Offset       int
}

type UploadOfferImageResponse struct {
	URL string `json:"url"`
}
