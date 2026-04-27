package orders

import (
	"kursach_backend/internal/domain"
	"kursach_backend/internal/notifications"
	"strconv"

	"github.com/google/uuid"
)

const (
	orderNotificationTypePaid      = "order_paid"
	orderNotificationTypeCancelled = "order_cancelled"
	orderNotificationTypeCompleted = "order_completed"
)

func orderPaidNotificationPayload(order *domain.Order) notifications.Payload {
	body := "Ваш заказ успешно оплачен!"
	if order.OrderNumber != nil {
		body = "Ваш заказ №" + *order.OrderNumber + " успешно оплачен!"
	}

	return notifications.Payload{
		Title:    "Заказ оплачен",
		Body:     body,
		DeepLink: orderDeepLink(order.ID),
		Data:     orderNotificationData(order, orderNotificationTypePaid),
	}
}

func orderCancelledNotificationPayload(order *domain.Order, refundAmount float64) notifications.Payload {
	body := "Ваш заказ был отменен."
	data := orderNotificationData(order, orderNotificationTypeCancelled)

	if refundAmount > 0 {
		body = "Ваш заказ был отменен. Средства будут возвращены."
		data["refundAmount"] = strconv.FormatFloat(refundAmount, 'f', 2, 64)
	}

	return notifications.Payload{
		Title:    "Заказ отменён",
		Body:     body,
		DeepLink: orderDeepLink(order.ID),
		Data:     data,
	}
}

func orderCompletedNotificationPayload(order *domain.Order) notifications.Payload {
	body := "Заказ выдан! Приятного аппетита."
	if order.OrderNumber != nil {
		body = "Заказ №" + *order.OrderNumber + " успешно выдан! Спасибо, что спасаете еду."
	}

	return notifications.Payload{
		Title:    "Заказ выдан",
		Body:     body,
		DeepLink: orderDeepLink(order.ID),
		Data:     orderNotificationData(order, orderNotificationTypeCompleted),
	}
}

func orderDeepLink(orderID uuid.UUID) string {
	return "/orders/" + orderID.String()
}

func orderNotificationData(order *domain.Order, notificationType string) map[string]string {
	data := map[string]string{
		"type":    notificationType,
		"orderId": order.ID.String(),
		"status":  string(order.Status),
		"offerId": order.OfferID.String(),
	}

	if order.Offer.RestaurantID != uuid.Nil {
		data["restaurantId"] = order.Offer.RestaurantID.String()
	}
	if order.OrderNumber != nil {
		data["orderNumber"] = *order.OrderNumber
	}

	return data
}
