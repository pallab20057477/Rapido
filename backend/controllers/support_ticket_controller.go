package controllers

import (
	"net/http"
	"strconv"

	"rapido-backend/models"
	"rapido-backend/services"
	"rapido-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SupportTicketController handles support ticket requests
type SupportTicketController struct {
	service *services.SupportTicketService
}

// NewSupportTicketController creates a new controller
func NewSupportTicketController() *SupportTicketController {
	return &SupportTicketController{
		service: services.NewSupportTicketService(),
	}
}

// CreateTicketRequest represents request to create ticket
type CreateTicketRequest struct {
	Category    string `json:"category" binding:"required"`
	Priority    string `json:"priority"`
	Subject     string `json:"subject" binding:"required"`
	Description string `json:"description" binding:"required"`
	RideID      string `json:"ride_id"`
}

// CreateTicket creates a new support ticket
// POST /api/v1/users/support/tickets
func (c *SupportTicketController) CreateTicket(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	uid, _ := uuid.Parse(userID)
	userType := ctx.GetString("userType")

	var req CreateTicketRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	// Convert to models request
	ticketReq := models.SupportTicketRequest{
		Category:    req.Category,
		Priority:    req.Priority,
		Subject:     req.Subject,
		Description: req.Description,
		RideID:      req.RideID,
	}

	ticket, err := c.service.CreateTicket(uid, userType, ticketReq)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Failed to create ticket", err.Error()))
		return
	}

	ctx.JSON(http.StatusCreated, utils.SuccessResponse("Support ticket created successfully", ticket))
}

// GetMyTickets gets the user's support tickets
// GET /api/v1/users/support/tickets
func (c *SupportTicketController) GetMyTickets(ctx *gin.Context) {
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

	tickets, total, err := c.service.GetUserTickets(uid, page, limit)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to get tickets", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Tickets retrieved", gin.H{
		"tickets": tickets,
		"meta": gin.H{
			"page":        page,
			"per_page":    limit,
			"total":       total,
			"total_pages": (total + int64(limit) - 1) / int64(limit),
		},
	}))
}

// GetTicketDetails gets ticket details with messages
// GET /api/v1/users/support/tickets/:id
func (c *SupportTicketController) GetTicketDetails(ctx *gin.Context) {
	ticketID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid ticket ID", err.Error()))
		return
	}

	ticket, messages, err := c.service.GetTicketByID(ticketID)
	if err != nil {
		ctx.JSON(http.StatusNotFound, utils.ErrorResponse("Ticket not found", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Ticket details retrieved", gin.H{
		"ticket":   ticket,
		"messages": messages,
	}))
}

// AddMessageRequest represents request to add message
type AddMessageRequest struct {
	Message string `json:"message" binding:"required"`
}

// AddMessage adds a message to a ticket
// POST /api/v1/users/support/tickets/:id/messages
func (c *SupportTicketController) AddMessage(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	uid, _ := uuid.Parse(userID)
	userType := ctx.GetString("userType")

	ticketID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid ticket ID", err.Error()))
		return
	}

	var req AddMessageRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	message, err := c.service.AddMessage(ticketID, uid, userType, req.Message, false)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Failed to add message", err.Error()))
		return
	}

	ctx.JSON(http.StatusCreated, utils.SuccessResponse("Message added successfully", message))
}

// AdminGetAllTickets gets all tickets (admin only)
// GET /api/v1/admin/support/tickets
func (c *SupportTicketController) AdminGetAllTickets(ctx *gin.Context) {
	status := ctx.Query("status")
	category := ctx.Query("category")

	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(ctx.DefaultQuery("per_page", "10"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	tickets, total, err := c.service.AdminGetAllTickets(status, category, page, limit)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to get tickets", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("All tickets retrieved", gin.H{
		"tickets": tickets,
		"meta": gin.H{
			"page":        page,
			"per_page":    limit,
			"total":       total,
			"total_pages": (total + int64(limit) - 1) / int64(limit),
		},
	}))
}

// AdminUpdateTicketRequest represents admin update request
type AdminUpdateTicketRequest struct {
	Status       string  `json:"status"`
	Priority     string  `json:"priority"`
	AssignedTo   string  `json:"assigned_to"`
	Resolution   string  `json:"resolution"`
	RefundAmount float64 `json:"refund_amount"`
}

// AdminUpdateTicket updates a ticket (admin only)
// PUT /api/v1/admin/support/tickets/:id
func (c *SupportTicketController) AdminUpdateTicket(ctx *gin.Context) {
	adminID := ctx.GetString("userID")
	adminUID, _ := uuid.Parse(adminID)

	ticketID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid ticket ID", err.Error()))
		return
	}

	var req AdminUpdateTicketRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	updates := make(map[string]interface{})
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if req.Priority != "" {
		updates["priority"] = req.Priority
	}
	if req.AssignedTo != "" {
		updates["assigned_to"] = req.AssignedTo
	}
	if req.Resolution != "" {
		updates["resolution"] = req.Resolution
	}
	if req.RefundAmount > 0 {
		updates["refund_amount"] = req.RefundAmount
		updates["refund_status"] = "pending"
	}

	if err := c.service.AdminUpdateTicket(ticketID, adminUID, updates); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Failed to update ticket", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Ticket updated successfully", nil))
}

// AdminAddMessageRequest represents admin message request
type AdminAddMessageRequest struct {
	Message    string `json:"message" binding:"required"`
	IsInternal bool   `json:"is_internal"`
}

// AdminAddMessage adds message to ticket (admin only)
// POST /api/v1/admin/support/tickets/:id/messages
func (c *SupportTicketController) AdminAddMessage(ctx *gin.Context) {
	adminID := ctx.GetString("userID")
	adminUID, _ := uuid.Parse(adminID)

	ticketID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid ticket ID", err.Error()))
		return
	}

	var req AdminAddMessageRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	message, err := c.service.AddMessage(ticketID, adminUID, "admin", req.Message, req.IsInternal)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Failed to add message", err.Error()))
		return
	}

	ctx.JSON(http.StatusCreated, utils.SuccessResponse("Message added successfully", message))
}
