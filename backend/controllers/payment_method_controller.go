package controllers

import (
	"net/http"

	"rapido-backend/models"
	"rapido-backend/services"
	"rapido-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// PaymentMethodController handles payment method requests
type PaymentMethodController struct {
	service *services.PaymentMethodService
}

// NewPaymentMethodController creates new controller
func NewPaymentMethodController() *PaymentMethodController {
	return &PaymentMethodController{
		service: services.NewPaymentMethodService(),
	}
}

// AddCardRequest represents request to add a card
type AddCardRequest struct {
	CardNumber     string             `json:"card_number" binding:"required"`
	ExpiryMonth    int                `json:"expiry_month" binding:"required,min=1,max=12"`
	ExpiryYear     int                `json:"expiry_year" binding:"required,min=2024,max=2040"`
	CVV            string             `json:"cvv" binding:"required,len=3"`
	CardholderName string             `json:"cardholder_name"`
	CardType       models.CardType    `json:"card_type" binding:"required,oneof=credit debit"`
	Nickname       string             `json:"nickname"`
	SetAsDefault   bool               `json:"set_as_default"`
	BillingAddress string             `json:"billing_address"`
}

// AddCard adds a new saved card
// POST /api/v1/payments/methods/card
func (c *PaymentMethodController) AddCard(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	uid, _ := uuid.Parse(userID)

	var req AddCardRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	// Convert to AddPaymentMethodRequest
	pmReq := models.AddPaymentMethodRequest{
		Type:           models.PaymentMethodCard,
		CardNumber:     req.CardNumber,
		ExpiryMonth:    req.ExpiryMonth,
		ExpiryYear:     req.ExpiryYear,
		CVV:            req.CVV,
		CardholderName: req.CardholderName,
		CardType:       req.CardType,
		Nickname:       req.Nickname,
		SetAsDefault:   req.SetAsDefault,
		BillingAddress: req.BillingAddress,
	}

	pm, err := c.service.AddCard(uid, pmReq)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Failed to add card", err.Error()))
		return
	}

	resp := c.service.ToResponse(*pm)
	ctx.JSON(http.StatusCreated, utils.SuccessResponse("Card added successfully", resp))
}

// AddUPIRequest represents request to add UPI
type AddUPIRequest struct {
	VPA          string `json:"vpa" binding:"required"`
	Nickname     string `json:"nickname"`
	SetAsDefault bool   `json:"set_as_default"`
}

// AddUPI adds a new saved UPI ID
// POST /api/v1/payments/methods/upi
func (c *PaymentMethodController) AddUPI(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	uid, _ := uuid.Parse(userID)

	var req AddUPIRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	pmReq := models.AddPaymentMethodRequest{
		Type:         models.PaymentMethodUPI,
		VPA:          req.VPA,
		Nickname:     req.Nickname,
		SetAsDefault: req.SetAsDefault,
	}

	pm, err := c.service.AddUPI(uid, pmReq)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Failed to add UPI", err.Error()))
		return
	}

	resp := c.service.ToResponse(*pm)
	ctx.JSON(http.StatusCreated, utils.SuccessResponse("UPI added successfully", resp))
}

// GetPaymentMethods gets all saved payment methods
// GET /api/v1/payments/methods
func (c *PaymentMethodController) GetPaymentMethods(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	uid, _ := uuid.Parse(userID)

	methods, err := c.service.GetPaymentMethods(uid)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to get payment methods", err.Error()))
		return
	}

	// Convert to responses
	responses := make([]models.PaymentMethodResponse, 0, len(methods))
	for _, pm := range methods {
		responses = append(responses, c.service.ToResponse(pm))
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Payment methods retrieved", responses))
}

// RemovePaymentMethod removes a saved payment method
// DELETE /api/v1/payments/methods/:id
func (c *PaymentMethodController) RemovePaymentMethod(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	uid, _ := uuid.Parse(userID)

	methodID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid method ID", err.Error()))
		return
	}

	if err := c.service.RemovePaymentMethod(methodID, uid); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Failed to remove payment method", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Payment method removed successfully", nil))
}

// SetDefaultPaymentMethod sets a payment method as default
// POST /api/v1/payments/methods/:id/default
func (c *PaymentMethodController) SetDefaultPaymentMethod(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	uid, _ := uuid.Parse(userID)

	methodID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid method ID", err.Error()))
		return
	}

	if err := c.service.SetDefaultPaymentMethod(methodID, uid); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Failed to set default", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Default payment method set successfully", nil))
}
