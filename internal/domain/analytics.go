package domain

import (
	"time"

	"github.com/google/uuid"
)

type DailyAnalytics struct {
	ID              uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	RestaurantID    uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_rest_date_cat" json:"restaurant_id"`
	Date            time.Time `gorm:"type:date;uniqueIndex:idx_rest_date_cat" json:"date"`
	CategoryName    string    `gorm:"uniqueIndex:idx_rest_date_cat" json:"category_name"`
	TotalBookings   int       `json:"total_bookings"`
	CompletedOrders int       `json:"completed_orders"`
	CancelledOrders int       `json:"cancelled_orders"`
	ExpiredOrders   int       `json:"expired_orders"`
	GrossRevenue    float64   `gorm:"type:decimal(10,2)" json:"gross_revenue"`
	ServiceFee      float64   `gorm:"type:decimal(10,2)" json:"service_fee"`
	NetPayout       float64   `gorm:"type:decimal(10,2)" json:"net_payout"`
	CreatedAt       time.Time `json:"created_at"`
}
