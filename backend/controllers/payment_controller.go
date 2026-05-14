package controllers

import (
	"net/http"
	"strconv"

	"rapido-backend/services"
	"rapido-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type PaymentController struct {
	Service *services.PaymentService
}

func NewPaymentController() *PaymentController {
	return &PaymentController{Service: services.NewPaymentService()}
}

// ProcessPaymentRequest request body
type ProcessPaymentRequest struct {
	Method string `json:"method" binding:"required"`
}

type RetryPaymentRequest struct {
	Method string `json:"method,omitempty"`
}

type RefundPaymentRequest struct {
	Amount float64 `json:"amount" binding:"required,gt=0"`
	Reason string  `json:"reason,omitempty"`
}

// ProcessPayment processes payment for a ride
func (c *PaymentController) ProcessPayment(ctx *gin.Context) {
	rideID := ctx.Param("id")
	rid, err := uuid.Parse(rideID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid ride ID", ""))
		return
	}

	var req ProcessPaymentRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	idempotencyKey := ctx.GetHeader("Idempotency-Key")
	if idempotencyKey == "" {
		idempotencyKey = utils.GenerateIdempotencyKey()
	}

	payment, err := c.Service.ProcessPayment(rid, req.Method, idempotencyKey)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Payment failed", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Payment processed", payment))
}

// RetryPayment retries a previously failed payment for a ride
func (c *PaymentController) RetryPayment(ctx *gin.Context) {
	rideID := ctx.Param("id")
	rid, err := uuid.Parse(rideID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid ride ID", ""))
		return
	}

	var req RetryPaymentRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		// If body is empty it's fine (method optional), but malformed JSON should be rejected
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	idempotencyKey := ctx.GetHeader("Idempotency-Key")
	if idempotencyKey == "" {
		idempotencyKey = utils.GenerateIdempotencyKey()
	}

	payment, err := c.Service.RetryFailedPayment(rid, req.Method, idempotencyKey)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Retry payment failed", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Payment retry processed", payment))
}

// RefundPayment refunds a completed payment back to rider wallet
func (c *PaymentController) RefundPayment(ctx *gin.Context) {
	paymentID := ctx.Param("id")
	pid, err := uuid.Parse(paymentID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid payment ID", ""))
		return
	}

	userID := ctx.GetString("userID")
	uid, err := uuid.Parse(userID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid user ID", ""))
		return
	}

	var req RefundPaymentRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	payment, err := c.Service.RefundPayment(pid, uid, req.Amount, req.Reason)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Refund failed", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Refund processed", payment))
}

// GetPaymentStatus gets payment status for a ride
func (c *PaymentController) GetPaymentStatus(ctx *gin.Context) {
	rideID := ctx.Param("id")
	rid, err := uuid.Parse(rideID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid ride ID", ""))
		return
	}

	payment, err := c.Service.GetPaymentByRide(rid)
	if err != nil {
		ctx.JSON(http.StatusNotFound, utils.ErrorResponse("Payment not found", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Payment status", payment))
}

// GetWallet gets user's wallet
func (c *PaymentController) GetWallet(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	uid, _ := uuid.Parse(userID)

	wallet, err := c.Service.GetWallet(uid)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to fetch wallet", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Wallet retrieved", wallet))
}

// AddMoneyRequest request body
type AddMoneyRequest struct {
	Amount float64 `json:"amount" binding:"required,gt=0"`
	Method string  `json:"method" binding:"required"`
}

// AddMoneyToWallet adds money to wallet
func (c *PaymentController) AddMoneyToWallet(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	uid, _ := uuid.Parse(userID)

	var req AddMoneyRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	transaction, err := c.Service.AddMoneyToWallet(uid, req.Amount, req.Method)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Failed to add money", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Money added successfully", transaction))
}

// GetTransactionHistory gets transaction history
func (c *PaymentController) GetTransactionHistory(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	uid, _ := uuid.Parse(userID)

	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(ctx.DefaultQuery("per_page", "10"))

	transactions, total, err := c.Service.GetTransactionHistory(uid, page, perPage)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to fetch transactions", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.PaginatedResponse(transactions, page, perPage, total))
}

// RequestWithdrawalRequest request body
type RequestWithdrawalRequest struct {
	Amount      float64                `json:"amount" binding:"required,gt=0"`
	Method      string                 `json:"method" binding:"required"`
	BankDetails map[string]interface{} `json:"bank_details,omitempty"`
}

// RequestWithdrawal requests a withdrawal
func (c *PaymentController) RequestWithdrawal(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	uid, _ := uuid.Parse(userID)

	var req RequestWithdrawalRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	withdrawal, err := c.Service.RequestWithdrawal(uid, req.Amount, req.Method, req.BankDetails)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Withdrawal request failed", err.Error()))
		return
	}

	ctx.JSON(http.StatusCreated, utils.SuccessResponse("Withdrawal request submitted", withdrawal))
}

// HandlePaymentWebhook handles payment webhooks from Razorpay/Stripe
func (c *PaymentController) HandlePaymentWebhook(ctx *gin.Context) {
	// Get webhook signature from header
	signature := ctx.GetHeader("X-Razorpay-Signature")
	if signature == "" {
		signature = ctx.GetHeader("Stripe-Signature")
	}

	// Read body
	body, err := ctx.GetRawData()
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Failed to read webhook body", ""))
		return
	}

	// Process webhook
	event, err := c.Service.ProcessWebhook(signature, body)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Webhook processing failed", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Webhook processed", event))
}
