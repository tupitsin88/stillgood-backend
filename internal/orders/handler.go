package orders

import (
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/gin-gonic/gin"
)

type OrderHandler struct {
	service *OrderService
}

func NewOrderHandler(service *OrderService) *OrderHandler {
	return &OrderHandler{service: service}
}

func errorResponse(c *gin.Context, code int, errorCode string, message string) {
	c.JSON(code, gin.H{
		"error":   errorCode,
		"message": message,
	})
}

// CreateOrder @Summary Создание заказа
// @Tags Orders
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param input body CreateOrderRequest true "Данные заказа"
// @Success 201 {object} CreateOrderResponse
// @Failure 400 {object} map[string]string
// @Router /orders [post]
func (h *OrderHandler) CreateOrder(c *gin.Context) {
	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, 400, "INVALID_REQUEST", err.Error())
		return
	}
	uidStr := c.GetString("user_id")
	userID, err := uuid.Parse(uidStr)
	if err != nil {
		errorResponse(c, 401, "UNAUTHORIZED", "Invalid User ID")
		return
	}

	order, err := h.service.CreateOrder(c.Request.Context(), userID, req)
	if err != nil {
		if strings.ToLower(err.Error()) == "offer not found" {
			errorResponse(c, 404, "OFFER_NOT_FOUND", "The requested offer does not exist")
		} else {
			errorResponse(c, 400, "CREATION_FAILED", err.Error())
		}
		return
	}

	resp := CreateOrderResponse{
		ID:        order.ID.String(),
		Status:    string(order.Status),
		Amount:    order.Amount,
		ExpiresAt: order.ExpiresAt,
		Offer: OfferShortDTO{
			ID:          order.Offer.ID.String(),
			Title:       order.Offer.Title,
			PickupStart: order.Offer.PickupStart,
			PickupEnd:   order.Offer.PickupEnd,
		},
		Restaurant: RestaurantSimpleDTO{
			Name:    order.Offer.Restaurant.Name,
			Address: order.Offer.Restaurant.Address,
		},
	}
	c.JSON(201, resp)
}

// PayOrder @Summary Оплата заказа
// @Tags Orders
// @Security ApiKeyAuth
// @Produce json
// @Param id path string true "Order ID"
// @Success 200 {object} PayOrderResponse
// @Failure 401 {object} map[string]string
// @Router /orders/{id}/pay [post]
func (h *OrderHandler) PayOrder(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errorResponse(c, 400, "INVALID_ID", "Invalid ID format")
		return
	}
	uidStr := c.GetString("user_id")
	userID, err := uuid.Parse(uidStr)
	if err != nil {
		errorResponse(c, 401, "UNAUTHORIZED", "Invalid or missing User ID")
		return
	}
	order, err := h.service.PayOrder(c.Request.Context(), id, userID)
	if err != nil {
		switch err.Error() {
		case "ORDER_EXPIRED":
			errorResponse(c, 422, "ORDER_EXPIRED", "Order payment time has expired")
		case "INVALID_ORDER_STATUS":
			errorResponse(c, 400, "INVALID_ORDER_STATUS", "Order is not in CREATED status")
		case "unauthorized":
			errorResponse(c, 403, "FORBIDDEN", "You do not own this order")
		case "not found":
			errorResponse(c, 404, "ORDER_NOT_FOUND", "Order not found")
		default:
			errorResponse(c, 400, "PAYMENT_FAILED", err.Error())
		}
		return
	}

	resp := PayOrderResponse{
		ID:          order.ID.String(),
		Status:      string(order.Status),
		OrderNumber: *order.OrderNumber,
		PaidAt:      order.PaidAt,
	}
	c.JSON(200, resp)
}

// CancelOrder @Summary Отмена заказа
// @Tags Orders
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path string true "Order ID"
// @Param input body CancelOrderRequest true "Причина отмены"
// @Success 200 {object} CancelOrderResponse
// @Router /orders/{id}/cancel [post]
func (h *OrderHandler) CancelOrder(c *gin.Context) {
	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errorResponse(c, 400, "INVALID_ID", "Invalid ID format")
		return
	}
	role := c.GetString("role")
	userIDStr := c.GetString("user_id")
	userID, _ := uuid.Parse(userIDStr)
	var actorID uuid.UUID
	if role == "PARTNER" {
		restIDStr := c.GetString("restaurant_id")
		actorID, _ = uuid.Parse(restIDStr)
	} else {
		actorID = userID
	}
	var req CancelOrderRequest
	err = c.ShouldBindJSON(&req)
	if err != nil {
		return
	}
	order, refund, err := h.service.CancelOrder(c.Request.Context(), orderID, actorID, role, req.Reason)
	if err != nil {
		switch err.Error() {
		case "CANNOT_CANCEL":
			errorResponse(c, 400, "CANNOT_CANCEL", "Order status does not allow cancellation")
		case "CANCELLATION_WINDOW_CLOSED":
			errorResponse(c, 400, "CANCELLATION_WINDOW_CLOSED", "Cancellation period has passed")
		case "unauthorized":
			errorResponse(c, 403, "FORBIDDEN", "You do not own this order")
		default:
			errorResponse(c, 400, "CANCELLATION_FAILED", err.Error())
		}
		return
	}

	resp := CancelOrderResponse{
		ID:          order.ID.String(),
		Status:      string(order.Status),
		CancelledAt: order.CancelledAt,
	}
	if refund > 0 {
		resp.RefundAmount = &refund
	}
	c.JSON(200, resp)
}

// CompleteOrder @Summary Подтверждение выдачи
// @Tags Partner
// @Security ApiKeyAuth
// @Produce json
// @Param id path string true "Order ID"
// @Success 200 {object} map[string]interface{}
// @Router /partner/orders/{id}/complete [post]
func (h *OrderHandler) CompleteOrder(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errorResponse(c, 400, "INVALID_ID", "Invalid ID format")
		return
	}
	restIDStr := c.GetString("restaurant_id")
	restaurantID, err := uuid.Parse(restIDStr)
	if err != nil {
		errorResponse(c, 400, "INVALID_ID", "Invalid ID format")
		return
	}
	order, err := h.service.CompleteOrder(c.Request.Context(), id, restaurantID)
	if err != nil {
		switch err.Error() {
		case "unauthorized":
			errorResponse(c, 403, "FORBIDDEN", "This order belongs to another restaurant")
		case "INVALID_ORDER_STATUS":
			errorResponse(c, 400, "INVALID_ORDER_STATUS", "Can only complete PAID orders")
		case "not found":
			errorResponse(c, 404, "ORDER_NOT_FOUND", "Order not found")
		default:
			errorResponse(c, 400, "COMPLETION_FAILED", err.Error())
		}
		return
	}

	c.JSON(200, CompleteOrderResponse{
		ID:          order.ID,
		Status:      string(order.Status),
		CompletedAt: order.CompletedAt,
	})
}

// GetUserOrders @Summary Заказы пользователя
// @Tags Orders
// @Security ApiKeyAuth
// @Produce json
// @Param status query string false "Status"
// @Param limit query integer false "Limit"
// @Param offset query integer false "Offset"
// @Success 200 {object} map[string]interface{}
// @Router /orders [get]
func (h *OrderHandler) GetUserOrders(c *gin.Context) {
	uidStr := c.GetString("user_id")
	userID, err := uuid.Parse(uidStr)
	if err != nil {
		errorResponse(c, 401, "UNAUTHORIZED", "Invalid or missing User ID")
		return
	}
	limit := 20
	offset := 0
	if l := c.Query("limit"); l != "" {
		limit, _ = strconv.Atoi(l)
	}
	if o := c.Query("offset"); o != "" {
		offset, _ = strconv.Atoi(o)
	}
	statusStr := c.Query("status")
	var statuses []string
	if statusStr != "" {
		statuses = strings.Split(statusStr, ",")
	}

	orders, total, err := h.service.repo.GetUserOrders(c.Request.Context(), userID, limit, offset, statuses)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	data := make([]OrderPreviewDTO, len(orders))
	for i, o := range orders {
		data[i] = OrderPreviewDTO{
			ID:          o.ID.String(),
			Status:      string(o.Status),
			Amount:      o.Amount,
			OrderNumber: o.OrderNumber,
			CreatedAt:   o.CreatedAt,
			ExpiresAt:   o.ExpiresAt,
			PickupStart: o.Offer.PickupStart,
			PickupEnd:   o.Offer.PickupEnd,
			Offer: OfferPreviewInternalDTO{
				Title:    o.Offer.Title,
				ImageURL: o.Offer.ImageURL,
			},
			Restaurant: RestaurantSimpleDTO{
				Name:    o.Offer.Restaurant.Name,
				Address: o.Offer.Restaurant.Address,
			},
		}
	}

	c.JSON(200, gin.H{
		"data":       data,
		"pagination": gin.H{"total": total, "limit": limit, "offset": offset},
	})
}

// GetUserStats @Summary Статистика пользователя
// @Description Получение количества спасенных боксов и сэкономленных денег
// @Tags Profile
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} UserStatsResponse
// @Router /orders/me/stats [get]
func (h *OrderHandler) GetUserStats(c *gin.Context) {
	uidStr := c.GetString("user_id")
	userID, _ := uuid.Parse(uidStr)
	boxes, money, err := h.service.repo.GetUserStats(c.Request.Context(), userID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, UserStatsResponse{
		SavedBoxes: boxes,
		SavedMoney: money,
	})
}

// GetNotifications @Summary История уведомлений
// @Description Получение списка уведомлений пользователя с пагинацией
// @Tags Profile
// @Security ApiKeyAuth
// @Produce json
// @Param limit query int false "Лимит" default(20)
// @Param offset query int false "Смещение" default(0)
// @Success 200 {object} []domain.Notification
// @Router /orders/me/notifications [get]
func (h *OrderHandler) GetNotifications(c *gin.Context) {
	uidStr := c.GetString("user_id")
	userID, _ := uuid.Parse(uidStr)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	notifications, err := h.service.GetNotifications(c.Request.Context(), userID, limit, offset)
	if err != nil {
		errorResponse(c, 500, "INTERNAL_ERROR", err.Error())
		return
	}
	c.JSON(200, notifications)
}

// GetPartnerOrders @Summary Заказы партнёра
// @Tags Partner
// @Security ApiKeyAuth
// @Produce json
// @Param status query string false "Status" Enums(CREATED, PAID, COMPLETED, CANCELLED)
// @Param limit query integer false "Limit"
// @Param offset query integer false "Offset"
// @Success 200 {object} map[string]interface{}
// @Router /partner/orders [get]
func (h *OrderHandler) GetPartnerOrders(c *gin.Context) {
	restIDStr := c.GetString("restaurant_id")
	restaurantID, _ := uuid.Parse(restIDStr)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	statusStr := c.Query("status")
	var statuses []string
	if statusStr != "" {
		statuses = strings.Split(statusStr, ",")
	}
	orders, total, err := h.service.repo.GetPartnerOrdersWithTotal(c.Request.Context(), restaurantID, limit, offset, statuses)
	if err != nil {
		errorResponse(c, 500, "INTERNAL_ERROR", err.Error())
		return
	}

	data := make([]PartnerOrderDTO, len(orders))
	for i, o := range orders {
		num := ""
		if o.OrderNumber != nil {
			num = *o.OrderNumber
		}
		data[i] = PartnerOrderDTO{
			ID:           o.ID.String(),
			OrderNumber:  num,
			Status:       string(o.Status),
			Amount:       o.Amount,
			ServiceFee:   o.ServiceFee,
			NetPayout:    o.NetPayout,
			OfferTitle:   o.Offer.Title,
			CustomerName: o.User.Name,
			PickupStart:  o.Offer.PickupStart,
			PickupEnd:    o.Offer.PickupEnd,
			CreatedAt:    o.CreatedAt,
		}
	}

	c.JSON(200, gin.H{
		"data": data,
		"pagination": gin.H{
			"total":  total,
			"limit":  limit,
			"offset": offset,
		},
	})
}

// GetOrderById @Summary Детали заказа
// @Tags Orders
// @Security ApiKeyAuth
// @Produce json
// @Param id path string true "Order ID"
// @Success 200 {object} OrderDetailDTO
// @Failure 404 {object} map[string]string
// @Router /orders/{id} [get]
func (h *OrderHandler) GetOrderById(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errorResponse(c, 400, "INVALID_ID", "Invalid ID format")
		return
	}
	uidStr := c.GetString("user_id")
	userID, _ := uuid.Parse(uidStr)

	order, err := h.service.GetOrderById(c.Request.Context(), id, userID)
	if err != nil {
		switch err.Error() {
		case "unauthorized":
			errorResponse(c, 403, "FORBIDDEN", "You do not own this order")
		case "not found":
			errorResponse(c, 404, "ORDER_NOT_FOUND", "Order not found")
		default:
			errorResponse(c, 500, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	resp := OrderDetailDTO{
		ID:                 order.ID.String(),
		Status:             string(order.Status),
		Amount:             order.Amount,
		OrderNumber:        order.OrderNumber,
		CreatedAt:          order.CreatedAt,
		PaidAt:             order.PaidAt,
		CompletedAt:        order.CompletedAt,
		CancelledAt:        order.CancelledAt,
		ExpiresAt:          order.ExpiresAt,
		CancellationReason: order.CancellationReason,
		Offer: OfferDetailInternalDTO{
			ID:                order.Offer.ID.String(),
			Title:             order.Offer.Title,
			Price:             order.Offer.Price,
			OriginalPrice:     order.Offer.OriginalPrice,
			Discount:          0,
			Description:       order.Offer.Description,
			ImageURL:          order.Offer.ImageURL,
			RestaurantID:      order.Offer.RestaurantID.String(),
			RestaurantName:    order.Offer.Restaurant.Name,
			PickupStart:       order.Offer.PickupStart,
			PickupEnd:         order.Offer.PickupEnd,
			QuantityAvailable: order.Offer.QuantityAvailable,
		},
		Restaurant: RestaurantShortDTO{
			ID:        order.Offer.Restaurant.ID.String(),
			Name:      order.Offer.Restaurant.Name,
			Address:   order.Offer.Restaurant.Address,
			Latitude:  order.Offer.Restaurant.Latitude,
			Longitude: order.Offer.Restaurant.Longitude,
			Phone:     order.Offer.Restaurant.Phone,
		},
	}

	c.JSON(200, resp)
}
