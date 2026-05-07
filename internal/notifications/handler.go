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
	userID, _ := uuid.Parse(uidStr)

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

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
