package controllers

import (
	"net/http"

	"rapido-backend/services"
	"rapido-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// BulkAdminController handles bulk admin operations
type BulkAdminController struct {
	service *services.BulkAdminService
}

// NewBulkAdminController creates new controller
func NewBulkAdminController() *BulkAdminController {
	return &BulkAdminController{
		service: services.NewBulkAdminService(),
	}
}

// BulkVerifyDriversRequest represents bulk verification request
type BulkVerifyDriversRequest struct {
	DriverIDs []string `json:"driver_ids" binding:"required"`
	Notes     string   `json:"notes"`
}

// BulkVerifyDrivers bulk verifies drivers
// POST /api/v1/admin/bulk/verify-drivers
func (c *BulkAdminController) BulkVerifyDrivers(ctx *gin.Context) {
	adminID := ctx.GetString("userID")
	adminUID, _ := uuid.Parse(adminID)

	var req BulkVerifyDriversRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	// Convert string IDs to UUIDs
	driverIDs := make([]uuid.UUID, 0, len(req.DriverIDs))
	for _, id := range req.DriverIDs {
		uid, err := uuid.Parse(id)
		if err == nil {
			driverIDs = append(driverIDs, uid)
		}
	}

	result, err := c.service.BulkVerifyDrivers(adminUID, driverIDs, req.Notes)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Failed to verify drivers", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Bulk verification completed", result))
}

// BulkNotifyRequest represents bulk notification request
type BulkNotifyRequest struct {
	UserIDs  []string `json:"user_ids" binding:"required"`
	UserType string   `json:"user_type" binding:"required,oneof=rider driver"`
	Title    string   `json:"title" binding:"required"`
	Body     string   `json:"body" binding:"required"`
	Channels []string `json:"channels"`
}

// BulkNotify sends bulk notifications
// POST /api/v1/admin/bulk/notify
func (c *BulkAdminController) BulkNotify(ctx *gin.Context) {
	adminID := ctx.GetString("userID")
	adminUID, _ := uuid.Parse(adminID)

	var req BulkNotifyRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	// Convert string IDs to UUIDs
	userIDs := make([]uuid.UUID, 0, len(req.UserIDs))
	for _, id := range req.UserIDs {
		uid, err := uuid.Parse(id)
		if err == nil {
			userIDs = append(userIDs, uid)
		}
	}

	notification := services.BulkNotification{
		Title:    req.Title,
		Body:     req.Body,
		Channels: req.Channels,
	}

	result, err := c.service.BulkSendNotifications(adminUID, userIDs, req.UserType, notification)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Failed to send notifications", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Bulk notifications queued", result))
}

// BulkImportDriversRequest represents driver import request
type BulkImportDriversRequest struct {
	Drivers []services.ImportDriverData `json:"drivers" binding:"required"`
}

// BulkImportDrivers imports drivers in bulk
// POST /api/v1/admin/bulk/import-drivers
func (c *BulkAdminController) BulkImportDrivers(ctx *gin.Context) {
	adminID := ctx.GetString("userID")
	adminUID, _ := uuid.Parse(adminID)

	var req BulkImportDriversRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	result, err := c.service.BulkImportDrivers(adminUID, req.Drivers)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Failed to import drivers", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Bulk import completed", result))
}

// BulkUpdateDriverStatusRequest represents status update request
type BulkUpdateDriverStatusRequest struct {
	DriverIDs []string `json:"driver_ids" binding:"required"`
	Status    string   `json:"status" binding:"required,oneof=active inactive suspended"`
	Reason    string   `json:"reason"`
}

// BulkUpdateDriverStatus updates driver status in bulk
// POST /api/v1/admin/bulk/update-driver-status
func (c *BulkAdminController) BulkUpdateDriverStatus(ctx *gin.Context) {
	adminID := ctx.GetString("userID")
	adminUID, _ := uuid.Parse(adminID)

	var req BulkUpdateDriverStatusRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	// Convert string IDs to UUIDs
	driverIDs := make([]uuid.UUID, 0, len(req.DriverIDs))
	for _, id := range req.DriverIDs {
		uid, err := uuid.Parse(id)
		if err == nil {
			driverIDs = append(driverIDs, uid)
		}
	}

	result, err := c.service.BulkUpdateDriverStatus(adminUID, driverIDs, req.Status, req.Reason)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Failed to update driver status", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Bulk status update completed", result))
}
