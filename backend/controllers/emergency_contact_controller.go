package controllers

import (
	"net/http"
	"strconv"

	"rapido-backend/models"
	"rapido-backend/services"
	"rapido-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// EmergencyContactController handles emergency contact and SOS requests
type EmergencyContactController struct {
	service *services.EmergencyContactService
}

// NewEmergencyContactController creates a new controller
func NewEmergencyContactController() *EmergencyContactController {
	return &EmergencyContactController{
		service: services.NewEmergencyContactService(),
	}
}

// AddEmergencyContact adds a new emergency contact
// POST /api/v1/auth/emergency-contacts
func (c *EmergencyContactController) AddEmergencyContact(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	utils.Debug("AddEmergencyContact - extracting user context",
		zap.String("user_id", userID))

	if userID == "" {
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", "user ID not found in context"))
		return
	}

	uid, err := uuid.Parse(userID)
	if err != nil {
		utils.Warn("AddEmergencyContact - failed to parse userID",
			zap.String("user_id", userID),
			zap.Error(err))
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", "invalid user ID"))
		return
	}

	utils.Debug("AddEmergencyContact - user validated",
		zap.String("user_id", uid.String()))

	var req models.EmergencyContactRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	contact, err := c.service.AddEmergencyContact(uid, req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Failed to add contact", err.Error()))
		return
	}

	ctx.JSON(http.StatusCreated, utils.SuccessResponse("Emergency contact added successfully", contact))
}

// GetEmergencyContacts gets all emergency contacts for the user
// GET /api/v1/auth/emergency-contacts
func (c *EmergencyContactController) GetEmergencyContacts(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	uid, _ := uuid.Parse(userID)

	contacts, err := c.service.GetEmergencyContacts(uid)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to get contacts", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Emergency contacts retrieved", contacts))
}

// UpdateEmergencyContact updates an existing contact
// PUT /api/v1/auth/emergency-contacts/:id
func (c *EmergencyContactController) UpdateEmergencyContact(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	uid, _ := uuid.Parse(userID)

	contactID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid contact ID", err.Error()))
		return
	}

	var req models.EmergencyContactRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	contact, err := c.service.UpdateEmergencyContact(contactID, uid, req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Failed to update contact", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Emergency contact updated successfully", contact))
}

// RemoveEmergencyContact removes an emergency contact
// DELETE /api/v1/auth/emergency-contacts/:id
func (c *EmergencyContactController) RemoveEmergencyContact(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	uid, _ := uuid.Parse(userID)

	contactID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid contact ID", err.Error()))
		return
	}

	if err := c.service.RemoveEmergencyContact(contactID, uid); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Failed to remove contact", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Emergency contact removed successfully", nil))
}

// TriggerSOS triggers an emergency SOS alert
// POST /api/v1/sos/trigger
func (c *EmergencyContactController) TriggerSOS(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	uid, _ := uuid.Parse(userID)

	var req models.SOSRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	response, err := c.service.TriggerSOS(uid, req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Failed to trigger SOS", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("SOS triggered successfully", response))
}

// GetMySOSHistory gets the user's SOS history
// GET /api/v1/sos/history
func (c *EmergencyContactController) GetMySOSHistory(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	uid, _ := uuid.Parse(userID)

	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(ctx.DefaultQuery("per_page", "10"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	events, total, err := c.service.GetUserSOSEvents(uid, page, limit)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to get SOS history", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("SOS history retrieved", gin.H{
		"events": events,
		"meta": gin.H{
			"page":        page,
			"per_page":    limit,
			"total":       total,
			"total_pages": (total + int64(limit) - 1) / int64(limit),
		},
	}))
}

// AdminGetActiveSOSEvents gets all active SOS events (admin only)
// GET /api/v1/admin/sos/active
func (c *EmergencyContactController) AdminGetActiveSOSEvents(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(ctx.DefaultQuery("per_page", "10"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	events, total, err := c.service.GetActiveSOSEvents(page, limit)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to get SOS events", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Active SOS events retrieved", gin.H{
		"events": events,
		"meta": gin.H{
			"page":        page,
			"per_page":    limit,
			"total":       total,
			"total_pages": (total + int64(limit) - 1) / int64(limit),
		},
	}))
}

// AdminResolveSOS resolves an active SOS event (admin only)
// POST /api/v1/admin/sos/:id/resolve
func (c *EmergencyContactController) AdminResolveSOS(ctx *gin.Context) {
	adminID := ctx.GetString("userID")
	adminUID, _ := uuid.Parse(adminID)

	eventID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid event ID", err.Error()))
		return
	}

	var req struct {
		Notes string `json:"notes"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	if err := c.service.ResolveSOS(eventID, adminUID, req.Notes); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Failed to resolve SOS", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("SOS event resolved successfully", nil))
}
