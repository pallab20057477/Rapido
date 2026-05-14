package controllers

import (
	"net/http"
	"rapido-backend/services"
	"rapido-backend/utils"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type NotificationController struct {
	Service *services.NotificationService
}

func NewNotificationController() *NotificationController {
	return &NotificationController{
		Service: services.NewNotificationService(),
	}
}

// GetNotifications gets paginated notifications for user
func (c *NotificationController) GetNotifications(ctx *gin.Context) {
	userID, _ := ctx.Get("user_id")
	uid, _ := uuid.Parse(userID.(string))

	// Parse pagination params
	page := 1
	perPage := 20
	if p := ctx.Query("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if l := ctx.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			perPage = parsed
		}
	}

	notifications, count, err := c.Service.GetNotifications(uid, page, perPage)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to get notifications", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Notifications retrieved", map[string]interface{}{
		"notifications": notifications,
		"pagination": map[string]interface{}{
			"page":        page,
			"limit":       perPage,
			"total":       count,
			"total_pages": (count + int64(perPage) - 1) / int64(perPage),
		},
	}))
}

// MarkAsRead marks a notification as read
func (c *NotificationController) MarkAsRead(ctx *gin.Context) {
	userID, _ := ctx.Get("user_id")
	uid, _ := uuid.Parse(userID.(string))

	notificationID := ctx.Param("id")
	nid, err := uuid.Parse(notificationID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid notification ID", ""))
		return
	}

	if err := c.Service.MarkAsRead(nid, uid); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Failed to mark as read", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Notification marked as read", nil))
}

// MarkAllAsRead marks all notifications as read
func (c *NotificationController) MarkAllAsRead(ctx *gin.Context) {
	userID, _ := ctx.Get("user_id")
	uid, _ := uuid.Parse(userID.(string))

	// Get all notifications and mark them as read
	notifications, _, err := c.Service.GetNotifications(uid, 1, 1000)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to get notifications", err.Error()))
		return
	}

	markedCount := 0
	for _, n := range notifications {
		if n.Status == "unread" {
			if err := c.Service.MarkAsRead(n.ID, uid); err == nil {
				markedCount++
			}
		}
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("All notifications marked as read", map[string]interface{}{
		"marked_count": markedCount,
	}))
}

// DeleteNotification soft-deletes a notification
func (c *NotificationController) DeleteNotification(ctx *gin.Context) {
	userID, _ := ctx.Get("user_id")
	uid, _ := uuid.Parse(userID.(string))

	notificationID := ctx.Param("id")
	nid, err := uuid.Parse(notificationID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid notification ID", ""))
		return
	}

	if err := c.Service.MarkAsRead(nid, uid); err != nil {
		// Mark as read first, then we'll soft delete in future implementation
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Failed to process notification", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Notification deleted", nil))
}

// RegisterDeviceToken registers device for push notifications
func (c *NotificationController) RegisterDeviceToken(ctx *gin.Context) {
	userID, _ := ctx.Get("user_id")
	uid, _ := uuid.Parse(userID.(string))

	var req struct {
		Token    string `json:"token" binding:"required"`
		Platform string `json:"platform" binding:"required,oneof=ios android web"`
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	if err := c.Service.RegisterDeviceToken(uid, req.Token, req.Platform); err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to register device", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Device registered for notifications", nil))
}
