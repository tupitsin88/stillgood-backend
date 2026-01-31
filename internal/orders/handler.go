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

// CreateOrder godoc
// @Tags orders
// @Security ApiKeyAuth
// @Summary Create a new order
// @Accept json
// @Produce json
// @Param request body CreateOrderRequest true "Order Creation Request"
// @Success 201 {object} orders.CreateOrderResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/orders [post]
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
		if err.Error() == "Offer not found" {
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

// PayOrder godoc
// @Tags orders
// @Security ApiKeyAuth
// @Summary Pay for an order
// @Accept json
// @Produce json
// @Param id path string true "Order ID"
// @Success 200 {object} orders.PayOrderResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/orders/{id}/pay [post]
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

// CancelOrder godoc
// @Tags orders
// @Security ApiKeyAuth
// @Summary Cancel an order
// @Accept json
// @Produce json
// @Param id path string true "Order ID"
// @Param request body CancelOrderRequest true "Cancellation Reason"
// @Success 200 {object} orders.CancelOrderResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/orders/{id}/cancel [post]
func (h *OrderHandler) CancelOrder(c *gin.Context) {
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
	var req CancelOrderRequest
	c.ShouldBindJSON(&req)

	order, refund, err := h.service.CancelOrder(c.Request.Context(), id, userID, req.Reason)
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

// CompleteOrder godoc
// @Tags partner-orders
// @Security ApiKeyAuth
// @Summary Complete an order
// @Accept json
// @Produce json
// @Param id path string true "Order ID"
// @Success 200 {object} orders.CompleteOrderResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/partner/orders/{id}/complete [post]
func (h *OrderHandler) CompleteOrder(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errorResponse(c, 400, "INVALID_ID", "Invalid ID format")
		return
	}
	order, err := h.service.CompleteOrder(c.Request.Context(), id)
	if err != nil {
		switch err.Error() {
		case "INVALID_ORDER_STATUS":
			errorResponse(c, 400, "INVALID_ORDER_STATUS", "Can only complete PAID orders")
		case "not found":
			errorResponse(c, 404, "ORDER_NOT_FOUND", "Order not found")
		default:
			errorResponse(c, 400, "COMPLETION_FAILED", err.Error())
		}
		return
	}

	c.JSON(200, gin.H{
		"id":          order.ID,
		"status":      string(order.Status),
		"completedAt": order.CompletedAt,
	})
}

// GetUserOrders godoc
// @Tags orders
// @Security ApiKeyAuth
// @Summary Get user orders
// @Accept json
// @Produce json
// @Param limit query int false "Limit"
// @Param status query string false "Comma-separated statuses"
// @Success 200 {object} orders.GetUserOrdersResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/orders [get]
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

// GetPartnerOrders godoc
// @Tags partner-orders
// @Security ApiKeyAuth
// @Summary Get partner orders
// @Accept json
// @Produce json
// @Success 200 {object} orders.GetPartnerOrdersResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/partner/orders [get]
func (h *OrderHandler) GetPartnerOrders(c *gin.Context) {
	orders, err := h.service.repo.GetPartnerOrders(c.Request.Context(), 20, 0, []string{"PAID", "CREATED"})
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
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
			OfferTitle:   o.Offer.Title,
			CustomerName: o.User.Name,
			PickupStart:  o.Offer.PickupStart,
			PickupEnd:    o.Offer.PickupEnd,
			CreatedAt:    o.CreatedAt,
		}
	}

	c.JSON(200, gin.H{"data": data})
}

// GetOrderById godoc
// @Tags orders
// @Security ApiKeyAuth
// @Summary Get order details
// @Accept json
// @Produce json
// @Param id path string true "Order ID"
// @Success 200 {object} orders.OrderDetailDTO
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/orders/{id} [get]
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
