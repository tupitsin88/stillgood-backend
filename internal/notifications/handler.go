package notifications

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type NotificationsHandler struct {
	service Service
}

func NewNotificationsHandler(s Service) *NotificationsHandler {
	return &NotificationsHandler{service: s}
}

// GetMyNotifications @Summary История уведомлений пользователя
func (h *NotificationsHandler) GetMyNotifications(c *gin.Context) {
	uidStr := c.GetString("user_id")
	userID, err := uuid.Parse(uidStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "UNAUTHORIZED", "message": "Invalid User ID format"})
		return
	}

	limit, offset, ok := notificationsPaginationQuery(c, 20)
	if !ok {
		return
	}

	notifications, err := h.service.ListForUser(c.Request.Context(), userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": notifications,
		"pagination": gin.H{
			"limit":  limit,
			"offset": offset,
		},
	})
}

func notificationsPaginationQuery(c *gin.Context, defaultLimit int) (int, int, bool) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(defaultLimit)))
	if err != nil || limit <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_LIMIT", "message": "limit must be a positive integer"})
		return 0, 0, false
	}
	if limit > 100 {
		limit = 100
	}

	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil || offset < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_OFFSET", "message": "offset must be a non-negative integer"})
		return 0, 0, false
	}
	return limit, offset, true
}
