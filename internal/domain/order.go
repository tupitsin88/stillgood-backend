package domain

import (
	"time"

	"github.com/google/uuid"
)

type OrderStatus string

const (
	OrderCreated   OrderStatus = "CREATED"
	OrderPaid      OrderStatus = "PAID"
	OrderCompleted OrderStatus = "COMPLETED"
	OrderCancelled OrderStatus = "CANCELLED"
)

type Order struct {
	ID                 uuid.UUID   `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID             uuid.UUID   `gorm:"type:uuid;index" json:"user_id"`
	OfferID            uuid.UUID   `gorm:"type:uuid;index" json:"offer_id"`
	User               User        `gorm:"foreignKey:UserID" json:"-"`
	Offer              Offer       `gorm:"foreignKey:OfferID" json:"-"`
	OrderNumber        *string     `gorm:"type:varchar(20)" json:"order_number"`
	Status             OrderStatus `gorm:"type:varchar(20)" json:"status"`
	Amount             float64     `gorm:"type:decimal(10,2)" json:"amount"`
	CreatedAt          time.Time   `json:"created_at"`
	PaidAt             *time.Time  `json:"paid_at,omitempty"`
	CompletedAt        *time.Time  `json:"completed_at,omitempty"`
	CancelledAt        *time.Time  `json:"cancelled_at,omitempty"`
	ExpiresAt          *time.Time  `json:"expires_at,omitempty"`
	CancellationReason *string     `json:"cancellation_reason,omitempty"`
}

type OrderStatusHistory struct {
	ID        uuid.UUID   `gorm:"primaryKey" json:"id"`
	OrderID   uuid.UUID   `gorm:"index" json:"order_id"`
	Status    OrderStatus `json:"status"`
	ChangedAt time.Time   `json:"changed_at"`
}
