package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"rapido-backend/config"
	"rapido-backend/database"
	"rapido-backend/models"
)

// RazorpayWebhookService handles secure webhook processing
type RazorpayWebhookService struct {
	webhookSecret string
}

// NewRazorpayWebhookService creates a new webhook service
func NewRazorpayWebhookService() *RazorpayWebhookService {
	cfg := config.Get()
	return &RazorpayWebhookService{
		webhookSecret: cfg.Razorpay.WebhookSecret,
	}
}

func isWebhookDevMode() bool {
	env := strings.ToLower(strings.TrimSpace(config.Get().App.Environment))
	return env == "development" || env == "dev" || env == ""
}

// RazorpayWebhookPayload represents the webhook event
type RazorpayWebhookPayload struct {
	Event     string          `json:"event"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt int64           `json:"created_at"`
}

// PaymentEntity represents the payment data
type PaymentEntity struct {
	ID               string                 `json:"id"`
	Entity           string                 `json:"entity"`
	Amount           int                    `json:"amount"`
	Currency         string                 `json:"currency"`
	Status           string                 `json:"status"`
	OrderID          string                 `json:"order_id"`
	InvoiceID        string                 `json:"invoice_id"`
	International    bool                   `json:"international"`
	Method           string                 `json:"method"`
	AmountRefunded   int                    `json:"amount_refunded"`
	RefundStatus     string                 `json:"refund_status"`
	Captured         bool                   `json:"captured"`
	Description      string                 `json:"description"`
	CardID           string                 `json:"card_id"`
	Bank             string                 `json:"bank"`
	Wallet           string                 `json:"wallet"`
	VPA              string                 `json:"vpa"`
	Email            string                 `json:"email"`
	Contact          string                 `json:"contact"`
	Notes            map[string]string      `json:"notes"`
	Fee              int                    `json:"fee"`
	Tax              int                    `json:"tax"`
	ErrorCode        string                 `json:"error_code"`
	ErrorDescription string                 `json:"error_description"`
	ErrorSource      string                 `json:"error_source"`
	ErrorStep        string                 `json:"error_step"`
	ErrorReason      string                 `json:"error_reason"`
	AcquirerData     map[string]interface{} `json:"acquirer_data"`
	CreatedAt        int64                  `json:"created_at"`
}

// VerifySignature validates the Razorpay webhook signature
func (ws *RazorpayWebhookService) VerifySignature(body []byte, signature string) bool {
	if ws.webhookSecret == "" {
		if isWebhookDevMode() {
			log.Println("Warning: Webhook secret not configured; allowing webhook in development mode")
			return true
		}
		log.Println("Warning: Webhook secret not configured in non-development mode")
		return false
	}

	if strings.TrimSpace(signature) == "" {
		return false
	}

	// Razorpay signature format: HMAC-SHA256 of webhook body with secret
	h := hmac.New(sha256.New, []byte(ws.webhookSecret))
	h.Write(body)
	computedSignature := hex.EncodeToString(h.Sum(nil))

	return hmac.Equal([]byte(computedSignature), []byte(signature))
}

// ProcessWebhook handles the webhook event with idempotency
func (ws *RazorpayWebhookService) ProcessWebhook(payload RazorpayWebhookPayload, rawBody []byte, signature string) error {
	// 1. Verify signature
	if !ws.VerifySignature(rawBody, signature) {
		return errors.New("invalid webhook signature")
	}

	// 2. Check idempotency (prevent duplicate processing)
	idempotencyKey := fmt.Sprintf("webhook:%s:%d", payload.Event, payload.CreatedAt)
	if exists, _ := database.GetCache(idempotencyKey); exists == "processed" {
		log.Printf("Webhook already processed: %s", idempotencyKey)
		return nil // Already processed, return success
	}

	// 3. Process based on event type
	switch payload.Event {
	case "payment.captured":
		if err := ws.handlePaymentCaptured(payload.Payload); err != nil {
			return err
		}
	case "payment.failed":
		if err := ws.handlePaymentFailed(payload.Payload); err != nil {
			return err
		}
	case "order.paid":
		if err := ws.handleOrderPaid(payload.Payload); err != nil {
			return err
		}
	default:
		log.Printf("Unhandled webhook event: %s", payload.Event)
	}

	// 4. Mark as processed (24-hour expiry for idempotency)
	database.SetCache(idempotencyKey, "processed", 24*time.Hour)

	return nil
}

// handlePaymentCaptured processes successful payment
func (ws *RazorpayWebhookService) handlePaymentCaptured(payload json.RawMessage) error {
	var data struct {
		Payment PaymentEntity `json:"payment"`
	}
	if err := json.Unmarshal(payload, &data); err != nil {
		return err
	}

	payment := data.Payment

	// Find payment by gateway reference
	var dbPayment models.Payment
	if err := database.DB.Where("gateway_ref = ?", payment.OrderID).First(&dbPayment).Error; err != nil {
		return fmt.Errorf("payment not found: %s", payment.OrderID)
	}

	// Idempotency check - already captured?
	if dbPayment.Status == models.PaymentStatusCompleted {
		log.Printf("Payment %s already captured, skipping", dbPayment.ID)
		return nil
	}

	// Update payment status
	dbPayment.Status = models.PaymentStatusCompleted
	dbPayment.TransactionID = payment.ID
	dbPayment.UpdatedAt = time.Now()

	if err := database.DB.Save(&dbPayment).Error; err != nil {
		return err
	}

	// Update ride payment status
	database.DB.Model(&models.Ride{}).Where("id = ?", dbPayment.RideID).
		Update("payment_status", models.PaymentStatusCompleted)

	// Credit driver wallet if applicable
	if dbPayment.Method == "online" {
		ws.creditDriverEarnings(dbPayment)
	}

	// Queue invoice generation via callback (avoids import cycle)
	if SubmitJobCallback != nil {
		if err := SubmitJobCallback("generate_invoice", map[string]string{
			"payment_id": dbPayment.ID.String(),
		}); err != nil {
			log.Printf("[WEBHOOK] Failed to queue invoice job: %v", err)
		}
	}

	log.Printf("Payment captured: %s (Razorpay ID: %s)", dbPayment.ID, payment.ID)
	return nil
}

// handlePaymentFailed processes failed payment
func (ws *RazorpayWebhookService) handlePaymentFailed(payload json.RawMessage) error {
	var data struct {
		Payment PaymentEntity `json:"payment"`
	}
	if err := json.Unmarshal(payload, &data); err != nil {
		return err
	}

	// Find and update payment
	var dbPayment models.Payment
	if err := database.DB.Where("gateway_ref = ?", data.Payment.OrderID).First(&dbPayment).Error; err != nil {
		return err
	}

	dbPayment.Status = models.PaymentStatusFailed
	dbPayment.TransactionID = data.Payment.ID
	dbPayment.UpdatedAt = time.Now()

	// Store error details
	if data.Payment.ErrorDescription != "" {
		// Could store in a separate error_log field or notes
		log.Printf("Payment failed: %s - %s", data.Payment.ErrorCode, data.Payment.ErrorDescription)
	}

	return database.DB.Save(&dbPayment).Error
}

// handleOrderPaid handles order completion
func (ws *RazorpayWebhookService) handleOrderPaid(payload json.RawMessage) error {
	// Handle order paid event if needed
	// Usually payment.captured handles the main logic
	return nil
}

// creditDriverEarnings credits the driver after payment confirmation
func (ws *RazorpayWebhookService) creditDriverEarnings(payment models.Payment) {
	// Find driver for this ride
	var ride models.Ride
	if err := database.DB.First(&ride, payment.RideID).Error; err != nil {
		return
	}

	if ride.DriverID == nil {
		return
	}

	// Get driver
	var driver models.Driver
	if err := database.DB.First(&driver, ride.DriverID).Error; err != nil {
		return
	}

	// Calculate commission
	commissionRate := 0.20 // 20% platform commission
	commission := payment.Amount * commissionRate
	driverEarning := payment.Amount - commission

	// Update driver aggregate earnings
	var driverEarnings models.DriverEarnings
	if err := database.DB.Where("driver_id = ?", ride.DriverID).First(&driverEarnings).Error; err != nil {
		// Create new record if not exists
		driverEarnings = models.DriverEarnings{
			DriverID:        *ride.DriverID,
			TotalEarnings:   driverEarning,
			TotalRides:      1,
			CurrentBalance:  driverEarning,
			WeeklyEarnings:  driverEarning,
			MonthlyEarnings: driverEarning,
			LastUpdated:     time.Now(),
		}
		database.DB.Create(&driverEarnings)
	} else {
		// Update existing
		driverEarnings.TotalEarnings += driverEarning
		driverEarnings.TotalRides++
		driverEarnings.CurrentBalance += driverEarning
		driverEarnings.WeeklyEarnings += driverEarning
		driverEarnings.MonthlyEarnings += driverEarning
		driverEarnings.LastUpdated = time.Now()
		database.DB.Save(&driverEarnings)
	}

	// Create commission record for this ride
	commissionRecord := models.Commission{
		RideID:             ride.ID,
		DriverID:           *ride.DriverID,
		TotalFare:          payment.Amount,
		PlatformCommission: commission,
		DriverEarnings:     driverEarning,
		TaxAmount:          0,
		ServiceFee:         0,
		PlatformPercent:    commissionRate * 100,
	}
	database.DB.Create(&commissionRecord)

	log.Printf("Credited ₹%.2f to driver %s for ride %s (commission: ₹%.2f)",
		driverEarning, *ride.DriverID, ride.ID, commission)
}
