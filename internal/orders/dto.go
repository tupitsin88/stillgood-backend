package orders

import (
	"time"

	"github.com/google/uuid"
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
	ServiceFee   float64   `json:"serviceFee"`
	NetPayout    float64   `json:"netPayout"`
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

// чё-то no usages
type GetUserOrdersResponse struct {
	Data       []OrderPreviewDTO `json:"data"`
	Pagination Pagination        `json:"pagination"`
}

type Pagination struct {
	Total  int64 `json:"total"`
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
}

// чё-то no usages
type GetPartnerOrdersResponse struct {
	Data []PartnerOrderDTO `json:"data"`
}

type CompleteOrderResponse struct {
	ID          uuid.UUID  `json:"id"`
	Status      string     `json:"status"`
	CompletedAt *time.Time `json:"completedAt"`
}

type UserStatsResponse struct {
	SavedBoxes int     `json:"savedBoxes"`
	SavedMoney float64 `json:"savedMoney"`
}
type NotificationDTO struct {
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

type CreateReviewRequest struct {
	Rating  int    `json:"rating" binding:"required,min=1,max=5"`
	Comment string `json:"comment" binding:"max=500"`
}

type ReviewDTO struct {
	ID        string    `json:"id"`
	Rating    int       `json:"rating"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"createdAt"`
}
