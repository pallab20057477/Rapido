package services

import (
	"context"
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
	"rapido-backend/models"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type PaymentReconciliationService struct {
	db     *gorm.DB
	redis  *redis.Client
	config *config.Config
}

type RazorpayWebhookEvent struct {
	Event     string                 `json:"event"`
	Contains  string                 `json:"contains"`
	Payload   map[string]interface{} `json:"payload"`
	CreatedAt int64                  `json:"created_at"`
}

type RazorpayPayment struct {
	ID               string                 `json:"id"`
	Entity           string                 `json:"entity"`
	Amount           float64                `json:"amount"`
	Currency         string                 `json:"currency"`
	Status           string                 `json:"status"`
	OrderID          string                 `json:"order_id"`
	InvoiceID        string                 `json:"invoice_id"`
	International    map[string]interface{} `json:"international"`
	Method           string                 `json:"method"`
	AmountRefunded   float64                `json:"amount_refunded"`
	RefundStatus     string                 `json:"refund_status"`
	Captured         bool                   `json:"captured"`
	Description      string                 `json:"description"`
	Email            string                 `json:"email"`
	Contact          string                 `json:"contact"`
	Notes            map[string]interface{} `json:"notes"`
	Fee              float64                `json:"fee"`
	Tax              float64                `json:"tax"`
	ErrorCode        string                 `json:"error_code"`
	ErrorDescription string                 `json:"error_description"`
	CreatedAt        int64                  `json:"created_at"`
}

type ReconciliationReport struct {
	TotalPayments     int                  `json:"total_payments"`
	MatchedPayments   int                  `json:"matched_payments"`
	UnmatchedPayments int                  `json:"unmatched_payments"`
	FailedPayments    int                  `json:"failed_payments"`
	TotalAmount       float64              `json:"total_amount"`
	MatchedAmount     float64              `json:"matched_amount"`
	UnmatchedAmount   float64              `json:"unmatched_amount"`
	FailedAmount      float64              `json:"failed_amount"`
	Discrepancies     []PaymentDiscrepancy `json:"discrepancies"`
	ProcessedAt       time.Time            `json:"processed_at"`
}

type PaymentDiscrepancy struct {
	PaymentID      string  `json:"payment_id"`
	LocalAmount    float64 `json:"local_amount"`
	RazorpayAmount float64 `json:"razorpay_amount"`
	LocalStatus    string  `json:"local_status"`
	RazorpayStatus string  `json:"razorpay_status"`
	Type           string  `json:"type"` // "amount_mismatch", "status_mismatch", "missing_local", "missing_remote"
}

func NewPaymentReconciliationService(db *gorm.DB, redisClient *redis.Client, cfg *config.Config) *PaymentReconciliationService {
	return &PaymentReconciliationService{
		db:     db,
		redis:  redisClient,
		config: cfg,
	}
}

// VerifyWebhookSignature verifies Razorpay webhook signature
func (prs *PaymentReconciliationService) VerifyWebhookSignature(body []byte, signature string, secret string) bool {
	if secret == "" {
		log.Println("Webhook secret is empty, skipping verification")
		return false
	}

	// Calculate expected signature
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(body)
	expectedSignature := hex.EncodeToString(h.Sum(nil))

	// Razorpay sends signature as "razorpay_signature1=signature1|razorpay_signature2=signature2|..."
	signatures := strings.Split(signature, "|")

	for _, sig := range signatures {
		parts := strings.Split(sig, "=")
		if len(parts) == 2 && parts[0] == "razorpay_signature1" {
			return parts[1] == expectedSignature
		}
	}

	return false
}

// HandleWebhook processes payment webhook events (generic implementation)
func (prs *PaymentReconciliationService) HandleWebhook(eventType string, payload map[string]interface{}) error {
	log.Printf("Processing webhook event: %s", eventType)

	switch eventType {
	case "payment.completed":
		return prs.handlePaymentCompleted(payload)
	case "payment.failed":
		return prs.handlePaymentFailed(payload)
	case "refund.processed":
		return prs.handleRefundProcessed(payload)
	default:
		log.Printf("Unhandled webhook event type: %s", eventType)
		return nil
	}
}

func (prs *PaymentReconciliationService) handlePaymentCompleted(payload map[string]interface{}) error {
	raw, ok := payload["payment"]
	if !ok {
		return fmt.Errorf("payment key missing in payload")
	}

	paymentData, ok := raw.(map[string]interface{})
	if !ok {
		return fmt.Errorf("payment payload has unexpected shape")
	}

	paymentID, _ := paymentData["id"].(string)
	orderID, _ := paymentData["order_id"].(string)
	amountFloat, amountOk := paymentData["amount"].(float64)
	if paymentID == "" && orderID == "" {
		return fmt.Errorf("payment identifier missing in payload")
	}
	if !amountOk {
		return fmt.Errorf("payment.amount missing or invalid for %s", paymentID)
	}

	// Find the local payment record using gateway reference
	if prs.db == nil {
		return fmt.Errorf("database not initialized")
	}

	var payment models.Payment
	err := prs.db.Where("gateway_ref = ? OR transaction_id = ?", paymentID, orderID).First(&payment).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("Payment not found for payment %s, creating unknown record", paymentID)
			return prs.createUnknownPaymentRecord(paymentData)
		}
		return fmt.Errorf("failed to query payment: %w", err)
	}

	// Update payment status
	payment.Status = models.PaymentStatusCompleted
	payment.Amount = amountFloat

	if err := prs.db.Save(&payment).Error; err != nil {
		return fmt.Errorf("failed to update payment: %w", err)
	}

	// Process ride payment if applicable
	return prs.processRidePayment(payment.RideID, amountFloat)
}

func (prs *PaymentReconciliationService) handlePaymentFailed(payload map[string]interface{}) error {
	raw, ok := payload["payment"]
	if !ok {
		return fmt.Errorf("payment key missing in payload")
	}

	paymentData, ok := raw.(map[string]interface{})
	if !ok {
		return fmt.Errorf("payment payload has unexpected shape")
	}

	paymentID, _ := paymentData["id"].(string)
	orderID, _ := paymentData["order_id"].(string)
	errorCode, _ := paymentData["error_code"].(string)
	errorDesc, _ := paymentData["error_description"].(string)

	if prs.db == nil {
		return fmt.Errorf("database not initialized")
	}

	// Find and update local payment
	var payment models.Payment
	err := prs.db.Where("gateway_ref = ? OR transaction_id = ?", paymentID, orderID).First(&payment).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("Failed payment not found: %s", paymentID)
			return nil
		}
		return fmt.Errorf("failed to query payment: %w", err)
	}

	payment.Status = models.PaymentStatusFailed
	payment.FailureReason = fmt.Sprintf("%s: %s", errorCode, errorDesc)

	if err := prs.db.Save(&payment).Error; err != nil {
		return fmt.Errorf("failed to update failed payment: %w", err)
	}

	log.Printf("Payment %s failed: %s", paymentID, payment.FailureReason)
	return nil
}

func (prs *PaymentReconciliationService) handleRefundProcessed(payload map[string]interface{}) error {
	raw, ok := payload["refund"]
	if !ok {
		return fmt.Errorf("refund key missing in payload")
	}

	refundData, ok := raw.(map[string]interface{})
	if !ok {
		return fmt.Errorf("refund payload has unexpected shape")
	}

	paymentID, _ := refundData["payment_id"].(string)
	amount, amountOk := refundData["amount"].(float64)
	if paymentID == "" || !amountOk {
		return fmt.Errorf("refund payload missing required fields")
	}

	if prs.db == nil {
		return fmt.Errorf("database not initialized")
	}

	// Find the original payment
	var payment models.Payment
	if err := prs.db.Where("gateway_ref = ?", paymentID).First(&payment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("original payment not found for refund: %w", err)
		}
		return fmt.Errorf("failed to query original payment: %w", err)
	}

	// Update payment refund status
	payment.RefundAmount = amount
	payment.Status = models.PaymentStatusRefunded
	now := time.Now()
	payment.RefundedAt = &now
	if err := prs.db.Save(&payment).Error; err != nil {
		return fmt.Errorf("failed to update refunded payment: %w", err)
	}

	// Update wallet balance for refunds
	return prs.refundToWallet(payment.PayerID, amount, paymentID)
}

func (prs *PaymentReconciliationService) createUnknownPaymentRecord(paymentData map[string]interface{}) error {
	if prs.db == nil {
		return fmt.Errorf("database not initialized")
	}

	paymentID, _ := paymentData["id"].(string)
	orderID, _ := paymentData["order_id"].(string)
	amount, _ := paymentData["amount"].(float64)
	status, _ := paymentData["status"].(string)

	payment := models.Payment{
		TransactionID: paymentID,
		GatewayRef:    orderID,
		Amount:        amount,
		Status:        status,
		Gateway:       "payment_gateway",
		CreatedAt:     time.Now(),
	}

	if err := prs.db.Create(&payment).Error; err != nil {
		return fmt.Errorf("failed to create unknown payment record: %w", err)
	}

	log.Printf("Created unknown payment record for %s", paymentID)
	return nil
}

func (prs *PaymentReconciliationService) processRidePayment(rideID uuid.UUID, amount float64) error {
	if prs.db == nil {
		return fmt.Errorf("database not initialized")
	}

	var ride models.Ride
	if err := prs.db.First(&ride, rideID).Error; err != nil {
		return fmt.Errorf("ride not found: %w", err)
	}

	// Update ride payment status
	ride.PaymentStatus = models.PaymentStatusCompleted
	ride.FinalFare = amount
	if err := prs.db.Save(&ride).Error; err != nil {
		return fmt.Errorf("failed to save ride: %w", err)
	}

	if ride.DriverID != nil {
		ledger := NewLedgerService()
		driverPayout := amount - ride.PlatformFee
		if driverPayout < 0 {
			driverPayout = 0
		}
		if err := ledger.RecordRideSettlement(prs.db, models.LedgerAccountTypePaymentClearing, nil, ride.RiderID, *ride.DriverID, amount, driverPayout, ride.PlatformFee, rideID.String()); err != nil {
			return fmt.Errorf("failed to record ride settlement: %w", err)
		}
	}

	// Process driver earnings
	if ride.DriverID != nil {
		prs.processDriverEarnings(*ride.DriverID, amount, ride.VehicleType)
	}

	return nil
}

func (prs *PaymentReconciliationService) processDriverEarnings(driverID uuid.UUID, amount float64, vehicleType string) {
	if prs.db == nil {
		log.Printf("database not initialized, skipping driver earnings processing for %s", driverID)
		return
	}

	var fareConfig models.FareConfig
	if err := prs.db.Where("vehicle_type = ?", vehicleType).First(&fareConfig).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("Failed to load fare config for vehicle %s: %v", vehicleType, err)
		}
	}

	commissionRate := fareConfig.PlatformFee / 100.0
	commission := amount * commissionRate
	driverEarning := amount - commission

	// Update driver earnings
	var earnings models.DriverEarnings
	if err := prs.db.Where("driver_id = ?", driverID).First(&earnings).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("Failed to load earnings for driver %s: %v", driverID, err)
		}
		// initialize if not found
		earnings = models.DriverEarnings{DriverID: driverID, TotalEarnings: 0, TotalRides: 0, LastUpdated: time.Now()}
	}

	earnings.TotalEarnings += driverEarning
	earnings.TotalRides += 1
	earnings.LastUpdated = time.Now()

	if err := prs.db.Save(&earnings).Error; err != nil {
		log.Printf("Failed to save driver earnings for %s: %v", driverID, err)
	}

	// Create commission record
	commissionRecord := models.Commission{
		DriverID:           driverID,
		TotalFare:          amount,
		PlatformCommission: commission,
		DriverEarnings:     driverEarning,
		CreatedAt:          time.Now(),
	}

	if err := prs.db.Create(&commissionRecord).Error; err != nil {
		log.Printf("Failed to create commission record: %v", err)
	}
}

func (prs *PaymentReconciliationService) refundToWallet(userID uuid.UUID, amount float64, referenceID string) error {
	if prs.db == nil {
		return fmt.Errorf("database not initialized")
	}

	var wallet models.Wallet
	if err := prs.db.Where("user_id = ?", userID).First(&wallet).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("wallet not found: %w", err)
		}
		return fmt.Errorf("failed to query wallet: %w", err)
	}

	wallet.Balance += amount
	if err := prs.db.Save(&wallet).Error; err != nil {
		return fmt.Errorf("failed to update wallet: %w", err)
	}

	ledger := NewLedgerService()
	if err := ledger.RecordRefund(prs.db, userID, amount, referenceID); err != nil {
		return fmt.Errorf("failed to record refund in ledger: %w", err)
	}

	// Create refund transaction
	transaction := models.Transaction{
		UserID:      userID,
		Type:        models.TransactionTypeRefund,
		Amount:      amount,
		Description: "Refund",
		Status:      models.PaymentStatusCompleted,
		CreatedAt:   time.Now(),
	}

	return prs.db.Create(&transaction).Error
}

// SyncPayments synchronizes local payments with external payment gateways
func (prs *PaymentReconciliationService) SyncPayments() (*ReconciliationReport, error) {
	log.Println("Starting payment reconciliation...")

	report := &ReconciliationReport{
		ProcessedAt: time.Now(),
	}

	// Get payments from last 24 hours
	yesterday := time.Now().Add(-24 * time.Hour)
	var localPayments []models.Payment

	if err := prs.db.Where("created_at >= ? AND gateway_ref IS NOT NULL",
		yesterday).Find(&localPayments).Error; err != nil {
		return nil, fmt.Errorf("failed to get local payments: %w", err)
	}

	report.TotalPayments = len(localPayments)

	// TODO: implement remote gateway fetch and comparison. For now mark as unmatched until compared.
	report.MatchedPayments = 0
	report.UnmatchedPayments = report.TotalPayments

	log.Printf("Reconciliation completed: %d total, %d matched, %d unmatched",
		report.TotalPayments, report.MatchedPayments, report.UnmatchedPayments)

	return report, nil
}

// RunScheduledSync runs payment reconciliation on schedule
// RunScheduledSync runs payment reconciliation on schedule until ctx is done
func (prs *PaymentReconciliationService) RunScheduledSync(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour) // Run every hour
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping scheduled payment reconciliation")
			return
		case <-ticker.C:
			report, err := prs.SyncPayments()
			if err != nil {
				log.Printf("Scheduled payment reconciliation failed: %v", err)
				continue
			}

			// Store report for monitoring
			prs.storeReconciliationReport(report)
		}
	}
}

// Backwards-compatible wrapper: starts the scheduled sync in background with a cancellable context
func (prs *PaymentReconciliationService) RunScheduledSyncBackground() context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())
	go prs.RunScheduledSync(ctx)
	return cancel
}

func (prs *PaymentReconciliationService) storeReconciliationReport(report *ReconciliationReport) {
	if prs.redis == nil {
		return
	}

	ctx := context.Background()
	key := fmt.Sprintf("reconciliation_report:%d", time.Now().Unix())

	data, _ := json.Marshal(report)
	prs.redis.Set(ctx, key, data, 24*time.Hour)
}
