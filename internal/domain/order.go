package domain

import (
	"time"
)

type OrderStatus string

const (
	OrderCreated   OrderStatus = "CREATED"
	OrderPaid      OrderStatus = "PAID"
	OrderCompleted OrderStatus = "COMPLETED"
	OrderCancelled OrderStatus = "CANCELLED"
)

type Order struct {
	ID                 int         `gorm:"primaryKey" json:"id"`
	UserID             int         `gorm:"index" json:"user_id"`
	OfferID            int         `gorm:"index" json:"offer_id"`
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
	ID        int         `gorm:"primaryKey" json:"id"`
	OrderID   int         `gorm:"index" json:"order_id"`
	Status    OrderStatus `json:"status"`
	ChangedAt time.Time   `json:"changed_at"`
}
