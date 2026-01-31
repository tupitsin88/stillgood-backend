package orders

import (
	"time"
)

type CreateOrderRequest struct {
	OfferID string `json:"offerId" binding:"required"`
}

type PayOrderRequest struct {
	// PaymentMethod string `json:"paymentMethod"` // Если решим расширить
}

type CancelOrderRequest struct {
	Reason string `json:"reason"`
}

type CreateOrderResponse struct {
	ID         string              `json:"id"`
	Status     string              `json:"status"`
	Amount     float64             `json:"amount"`
	ExpiresAt  *time.Time          `json:"expiresAt"`
	Offer      OfferShortDTO       `json:"offer"`
	Restaurant RestaurantSimpleDTO `json:"restaurant"`
}

type PayOrderResponse struct {
	ID          string     `json:"id"`
	Status      string     `json:"status"`
	OrderNumber string     `json:"orderNumber"`
	PaidAt      *time.Time `json:"paidAt"`
}

type CancelOrderResponse struct {
	ID           string     `json:"id"`
	Status       string     `json:"status"`
	CancelledAt  *time.Time `json:"cancelledAt"`
	RefundAmount *float64   `json:"refundAmount,omitempty"`
}

type OrderPreviewDTO struct {
	ID          string                  `json:"id"`
	Status      string                  `json:"status"`
	Amount      float64                 `json:"amount"`
	OrderNumber *string                 `json:"orderNumber,omitempty"`
	Offer       OfferPreviewInternalDTO `json:"offer"`
	Restaurant  RestaurantSimpleDTO     `json:"restaurant"`
	PickupStart time.Time               `json:"pickupStart"`
	PickupEnd   time.Time               `json:"pickupEnd"`
	CreatedAt   time.Time               `json:"createdAt"`
	ExpiresAt   *time.Time              `json:"expiresAt,omitempty"`
}

type OrderDetailDTO struct {
	ID                 string                 `json:"id"`
	Status             string                 `json:"status"`
	Amount             float64                `json:"amount"`
	OrderNumber        *string                `json:"orderNumber,omitempty"`
	Offer              OfferDetailInternalDTO `json:"offer"`
	Restaurant         RestaurantShortDTO     `json:"restaurant"`
	CreatedAt          time.Time              `json:"createdAt"`
	PaidAt             *time.Time             `json:"paidAt,omitempty"`
	CompletedAt        *time.Time             `json:"completedAt,omitempty"`
	CancelledAt        *time.Time             `json:"cancelledAt,omitempty"`
	ExpiresAt          *time.Time             `json:"expiresAt,omitempty"`
	CancellationReason *string                `json:"cancellationReason,omitempty"`
}

type PartnerOrderDTO struct {
	ID           string    `json:"id"`
	OrderNumber  string    `json:"orderNumber"`
	Status       string    `json:"status"`
	Amount       float64   `json:"amount"`
	OfferTitle   string    `json:"offerTitle"`
	CustomerName string    `json:"customerName"`
	PickupStart  time.Time `json:"pickupStart"`
	PickupEnd    time.Time `json:"pickupEnd"`
	CreatedAt    time.Time `json:"createdAt"`
}

type OfferShortDTO struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	PickupStart time.Time `json:"pickupStart"`
	PickupEnd   time.Time `json:"pickupEnd"`
}

type OfferPreviewInternalDTO struct {
	Title    string  `json:"title"`
	ImageURL *string `json:"imageUrl,omitempty"`
}

type OfferDetailInternalDTO struct {
	ID                string    `json:"id"`
	Title             string    `json:"title"`
	Price             float64   `json:"price"`
	OriginalPrice     float64   `json:"originalPrice"`
	Discount          int       `json:"discount"`
	Description       string    `json:"description"`
	ImageURL          *string   `json:"imageUrl"`
	RestaurantID      string    `json:"restaurantId"`
	RestaurantName    string    `json:"restaurantName"`
	Distance          int       `json:"distance"`
	PickupStart       time.Time `json:"pickupStart"`
	PickupEnd         time.Time `json:"pickupEnd"`
	QuantityAvailable int       `json:"quantityAvailable"`
}

type RestaurantSimpleDTO struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

type RestaurantShortDTO struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Address   string  `json:"address"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Phone     *string `json:"phone,omitempty"`
}
