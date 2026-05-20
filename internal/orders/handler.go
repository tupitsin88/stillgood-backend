package orders

import (
	"io"
	"kursach_backend/internal/domain"
	"kursach_backend/internal/pkg/httputil"
	"net/http"
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
func (h *OrderHandler) CreateOrder(c *gin.Context) {
	var req CreateOrderRequest
	if !httputil.BindJSON(c, &req) {
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
		switch err.Error() {
		case "OFFER_NOT_FOUND":
			errorResponse(c, 404, "OFFER_NOT_FOUND", "The requested offer does not exist")
		case "OFFER_OUT_OF_STOCK", "OFFER_NOT_ACTIVE", "PICKUP_PERIOD_EXPIRED":
			errorResponse(c, 422, err.Error(), "Offer is unavailable for booking")
		case "INVALID_OFFER_ID":
			errorResponse(c, http.StatusBadRequest, "INVALID_OFFER_ID", "offerId must be a valid UUID")
		default:
			errorResponse(c, http.StatusInternalServerError, "CREATION_FAILED", "Failed to create order")
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
			errorResponse(c, http.StatusInternalServerError, "PAYMENT_FAILED", "Failed to pay order")
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
func (h *OrderHandler) CancelOrder(c *gin.Context) {
	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errorResponse(c, 400, "INVALID_ID", "Invalid ID format")
		return
	}
	role := c.GetString("role")
	userIDStr := c.GetString("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		errorResponse(c, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or missing User ID")
		return
	}
	var actorID uuid.UUID
	if role == "PARTNER" {
		restIDStr := c.GetString("restaurant_id")
		actorID, err = uuid.Parse(restIDStr)
		if err != nil {
			errorResponse(c, http.StatusBadRequest, "INVALID_RESTAURANT_ID", "Your account is not linked to a restaurant")
			return
		}
	} else {
		actorID = userID
	}
	var req CancelOrderRequest
	err = c.ShouldBindJSON(&req)
	if err != nil && err != io.EOF {
		code, message := httputil.BindingError(err, &req)
		errorResponse(c, http.StatusBadRequest, code, message)
		return
	}
	order, refund, err := h.service.CancelOrder(c.Request.Context(), orderID, actorID, role, req.Reason)
	if err != nil {
		errText := err.Error()
		switch {
		case errText == "not found":
			errorResponse(c, 404, "ORDER_NOT_FOUND", "Order not found")
		case errText == "CANNOT_CANCEL":
			errorResponse(c, 400, "CANNOT_CANCEL", "Order status does not allow cancellation")
		case errText == "CANCELLATION_WINDOW_CLOSED":
			errorResponse(c, 400, "CANCELLATION_WINDOW_CLOSED", "Cancellation period has passed")
		case strings.HasPrefix(errText, "unauthorized"):
			errorResponse(c, 403, "FORBIDDEN", "You do not own this order")
		default:
			errorResponse(c, http.StatusInternalServerError, "CANCELLATION_FAILED", "Failed to cancel order")
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
func (h *OrderHandler) CompleteOrder(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errorResponse(c, 400, "INVALID_ID", "Invalid ID format")
		return
	}
	restIDStr := c.GetString("restaurant_id")
	restaurantID, err := uuid.Parse(restIDStr)
	if err != nil {
		errorResponse(c, 400, "INVALID_RESTAURANT_ID", "Your account is not linked to a restaurant")
		return
	}
	order, err := h.service.CompleteOrder(c.Request.Context(), id, restaurantID)
	if err != nil {
		switch err.Error() {
		case "unauthorized":
			errorResponse(c, 403, "FORBIDDEN", "This order belongs to another restaurant")
		case "INVALID_ORDER_STATUS":
			errorResponse(c, 400, "INVALID_ORDER_STATUS", "Only PAID orders can be completed")
		case "not found":
			errorResponse(c, 404, "ORDER_NOT_FOUND", "Order not found")
		default:
			errorResponse(c, http.StatusInternalServerError, "COMPLETION_FAILED", "Failed to complete order")
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
func (h *OrderHandler) GetUserOrders(c *gin.Context) {
	uidStr := c.GetString("user_id")
	userID, err := uuid.Parse(uidStr)
	if err != nil {
		errorResponse(c, 401, "UNAUTHORIZED", "Invalid or missing User ID")
		return
	}
	limit, offset, ok := paginationQuery(c, 20)
	if !ok {
		return
	}
	statusStr := c.Query("status")
	var statuses []string
	if statusStr != "" {
		for _, s := range strings.Split(statusStr, ",") {
			s = strings.ToUpper(strings.TrimSpace(s))
			if isValidStatus(s) {
				statuses = append(statuses, s)
			} else {
				errorResponse(c, 400, "INVALID_STATUS", "Status "+s+" is not allowed")
				return
			}
		}
	}

	orders, total, err := h.service.repo.GetUserOrders(c.Request.Context(), userID, limit, offset, statuses)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch orders")
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
func (h *OrderHandler) GetUserStats(c *gin.Context) {
	uidStr := c.GetString("user_id")
	userID, err := uuid.Parse(uidStr)
	if err != nil {
		errorResponse(c, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or missing User ID")
		return
	}
	boxes, money, err := h.service.repo.GetUserStats(c.Request.Context(), userID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch user stats")
		return
	}
	c.JSON(200, UserStatsResponse{
		SavedBoxes: boxes,
		SavedMoney: money,
	})
}

// GetPartnerOrders @Summary Заказы партнёра
func (h *OrderHandler) GetPartnerOrders(c *gin.Context) {
	restIDStr := c.GetString("restaurant_id")
	restaurantID, err := uuid.Parse(restIDStr)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_RESTAURANT_ID", "Your account is not linked to a restaurant")
		return
	}
	limit, offset, ok := paginationQuery(c, 20)
	if !ok {
		return
	}
	statusStr := c.Query("status")
	var statuses []string
	if statusStr != "" {
		for _, s := range strings.Split(statusStr, ",") {
			s = strings.ToUpper(strings.TrimSpace(s))
			if isValidStatus(s) {
				statuses = append(statuses, s)
			} else {
				errorResponse(c, 400, "INVALID_STATUS", "Status "+s+" is not allowed")
				return
			}
		}
	}
	orders, total, err := h.service.repo.GetPartnerOrdersWithTotal(c.Request.Context(), restaurantID, limit, offset, statuses)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch partner orders")
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
		"data":       data,
		"pagination": gin.H{"total": total, "limit": limit, "offset": offset},
	})
}

// GetPartnerOrderByID @Summary Детали заказа партнёра
func (h *OrderHandler) GetPartnerOrderByID(c *gin.Context) {
	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_ID", "Invalid ID format")
		return
	}
	restIDStr := c.GetString("restaurant_id")
	restaurantID, err := uuid.Parse(restIDStr)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_RESTAURANT_ID", "Your account is not linked to a restaurant")
		return
	}

	order, err := h.service.GetPartnerOrderByID(c.Request.Context(), orderID, restaurantID)
	if err != nil {
		switch err.Error() {
		case "unauthorized":
			errorResponse(c, http.StatusForbidden, "FORBIDDEN", "This order belongs to another restaurant")
		case "not found":
			errorResponse(c, http.StatusNotFound, "ORDER_NOT_FOUND", "Order not found")
		default:
			errorResponse(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch order")
		}
		return
	}

	c.JSON(http.StatusOK, orderDetailDTO(order))
}

// GetOrderById @Summary Детали заказа
func (h *OrderHandler) GetOrderById(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errorResponse(c, 400, "INVALID_ID", "Invalid ID format")
		return
	}
	uidStr := c.GetString("user_id")
	userID, err := uuid.Parse(uidStr)
	if err != nil {
		errorResponse(c, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or missing User ID")
		return
	}

	order, err := h.service.GetOrderById(c.Request.Context(), id, userID)
	if err != nil {
		switch err.Error() {
		case "unauthorized":
			errorResponse(c, 403, "FORBIDDEN", "You do not own this order")
		case "not found":
			errorResponse(c, 404, "ORDER_NOT_FOUND", "Order not found")
		default:
			errorResponse(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch order")
		}
		return
	}

	c.JSON(200, orderDetailDTO(order))
}

func orderDetailDTO(order *domain.Order) OrderDetailDTO {
	return OrderDetailDTO{
		ID:                 order.ID.String(),
		Status:             string(order.Status),
		Amount:             order.Amount,
		OrderNumber:        order.OrderNumber,
		CustomerName:       order.User.Name,
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
}

// CreateReview @Summary Оставить отзыв
func (h *OrderHandler) CreateReview(c *gin.Context) {
	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errorResponse(c, 400, "INVALID_ID", "Invalid Order ID format")
		return
	}
	var req CreateReviewRequest
	if !httputil.BindJSON(c, &req) {
		return
	}

	uidStr := c.GetString("user_id")
	userID, err := uuid.Parse(uidStr)
	if err != nil {
		errorResponse(c, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or missing User ID")
		return
	}

	review, err := h.service.CreateReview(c.Request.Context(), orderID, userID, req)
	if err != nil {
		switch err.Error() {
		case "not found":
			errorResponse(c, 404, "ORDER_NOT_FOUND", "Order not found")
		case "ORDER_NOT_COMPLETED":
			errorResponse(c, 400, "ORDER_NOT_COMPLETED", "You can only review completed orders")
		case "unauthorized":
			errorResponse(c, 403, "FORBIDDEN", "You do not own this order")
		default:
			errorResponse(c, http.StatusInternalServerError, "REVIEW_FAILED", "Failed to create review")
		}
		return
	}

	c.JSON(201, ReviewDTO{
		ID:        review.ID.String(),
		Rating:    review.Rating,
		Comment:   review.Comment,
		CreatedAt: review.CreatedAt,
	})
}

func isValidStatus(s string) bool {
	switch domain.OrderStatus(s) {
	case domain.OrderCreated, domain.OrderPaid, domain.OrderCompleted, domain.OrderCancelled:
		return true
	}
	return false
}

func paginationQuery(c *gin.Context, defaultLimit int) (int, int, bool) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(defaultLimit)))
	if err != nil || limit <= 0 {
		errorResponse(c, http.StatusBadRequest, "INVALID_LIMIT", "limit must be a positive integer")
		return 0, 0, false
	}
	if limit > 100 {
		limit = 100
	}

	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil || offset < 0 {
		errorResponse(c, http.StatusBadRequest, "INVALID_OFFSET", "offset must be a non-negative integer")
		return 0, 0, false
	}
	return limit, offset, true
}
