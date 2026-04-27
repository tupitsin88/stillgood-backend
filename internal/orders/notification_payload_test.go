package orders

import (
	"kursach_backend/internal/domain"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestOrderPaidNotificationPayload(t *testing.T) {
	orderID := uuid.New()
	offerID := uuid.New()
	restaurantID := uuid.New()
	orderNumber := "123456"

	payload := orderPaidNotificationPayload(&domain.Order{
		ID:          orderID,
		OfferID:     offerID,
		OrderNumber: &orderNumber,
		Status:      domain.OrderPaid,
		Offer: domain.Offer{
			RestaurantID: restaurantID,
		},
	})

	assert.Equal(t, "Заказ оплачен", payload.Title)
	assert.Equal(t, "Ваш заказ №123456 успешно оплачен!", payload.Body)
	assert.Equal(t, "/orders/"+orderID.String(), payload.DeepLink)
	assert.Equal(t, map[string]string{
		"type":         orderNotificationTypePaid,
		"orderId":      orderID.String(),
		"status":       string(domain.OrderPaid),
		"offerId":      offerID.String(),
		"restaurantId": restaurantID.String(),
		"orderNumber":  orderNumber,
	}, payload.Data)
}

func TestOrderCancelledNotificationPayloadWithRefund(t *testing.T) {
	orderID := uuid.New()
	offerID := uuid.New()
	restaurantID := uuid.New()

	payload := orderCancelledNotificationPayload(&domain.Order{
		ID:      orderID,
		OfferID: offerID,
		Status:  domain.OrderCancelled,
		Offer: domain.Offer{
			RestaurantID: restaurantID,
		},
	}, 150.5)

	assert.Equal(t, "Заказ отменён", payload.Title)
	assert.Equal(t, "Ваш заказ был отменен. Средства будут возвращены.", payload.Body)
	assert.Equal(t, "/orders/"+orderID.String(), payload.DeepLink)
	assert.Equal(t, map[string]string{
		"type":         orderNotificationTypeCancelled,
		"orderId":      orderID.String(),
		"status":       string(domain.OrderCancelled),
		"offerId":      offerID.String(),
		"restaurantId": restaurantID.String(),
		"refundAmount": "150.50",
	}, payload.Data)
}

func TestOrderCancelledNotificationPayloadWithoutRefund(t *testing.T) {
	orderID := uuid.New()

	payload := orderCancelledNotificationPayload(&domain.Order{
		ID:     orderID,
		Status: domain.OrderCancelled,
	}, 0)

	assert.Equal(t, "Заказ отменён", payload.Title)
	assert.Equal(t, "Ваш заказ был отменен.", payload.Body)
	assert.Equal(t, "/orders/"+orderID.String(), payload.DeepLink)
	assert.Equal(t, orderNotificationTypeCancelled, payload.Data["type"])
	assert.NotContains(t, payload.Data, "refundAmount")
}

func TestOrderCompletedNotificationPayload(t *testing.T) {
	orderID := uuid.New()
	offerID := uuid.New()
	restaurantID := uuid.New()
	orderNumber := "654321"

	payload := orderCompletedNotificationPayload(&domain.Order{
		ID:          orderID,
		OfferID:     offerID,
		OrderNumber: &orderNumber,
		Status:      domain.OrderCompleted,
		Offer: domain.Offer{
			RestaurantID: restaurantID,
		},
	})

	assert.Equal(t, "Заказ выдан", payload.Title)
	assert.Equal(t, "Заказ №654321 успешно выдан! Спасибо, что спасаете еду.", payload.Body)
	assert.Equal(t, "/orders/"+orderID.String(), payload.DeepLink)
	assert.Equal(t, map[string]string{
		"type":         orderNotificationTypeCompleted,
		"orderId":      orderID.String(),
		"status":       string(domain.OrderCompleted),
		"offerId":      offerID.String(),
		"restaurantId": restaurantID.String(),
		"orderNumber":  orderNumber,
	}, payload.Data)
}
