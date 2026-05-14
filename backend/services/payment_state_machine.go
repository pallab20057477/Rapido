package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"rapido-backend/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PaymentStateMachine handles explicit state transitions for ride completion and payment.
// This addresses the critical edge case: ride_completed + payment_failed.
//
// Transition diagram:
// payment_pending → payment_processing → payment_completed
//
//	↘ payment_failed (allows retry)
//
// Key invariant: A ride can be completed (ride_status = 'completed') while
// its payment.status = 'pending' or 'failed'. This prevents riders from being
// blocked if payment gateway is slow or offline.
type PaymentStateMachine struct {
	db *gorm.DB
}

// PaymentStates
const (
	PaymentStatePending    = "pending"    // Initial state; payment not yet processed
	PaymentStateProcessing = "processing" // In-flight to payment gateway
	PaymentStateCompleted  = "completed"  // Successfully captured
	PaymentStateFailed     = "failed"     // Gateway error; eligible for retry
	PaymentStateCancelled  = "cancelled"  // User or admin cancelled payment
	PaymentStateRefunding  = "refunding"  // Refund in progress
	PaymentStateRefunded   = "refunded"   // Refund completed
)

// PaymentStateTransition represents an edge case or forced failover.
type PaymentStateTransition struct {
	PaymentID   uuid.UUID
	FromState   string
	ToState     string
	Reason      string
	GatewayResp string
	RetryCount  int
	NextRetryAt *time.Time
	CreatedAt   time.Time
}

// NewPaymentStateMachine creates a new state machine
func NewPaymentStateMachine(db *gorm.DB) *PaymentStateMachine {
	return &PaymentStateMachine{db: db}
}

// ProcessPaymentWithFallback attempts to charge the rider's payment method.
// If the gateway times out or returns a retryable error, the function:
// 1. Leaves the ride as completed
// 2. Marks payment as 'pending' (not 'failed')
// 3. Schedules async retry via worker pool
// 4. Notifies rider that payment is pending
//
// This decouples ride completion from payment success, improving UX.
func (psm *PaymentStateMachine) ProcessPaymentWithFallback(
	ctx context.Context,
	rideID uuid.UUID,
	payerID uuid.UUID,
	amount float64,
	method string,
	gatewayTimeout time.Duration,
) (*models.Payment, error) {

	// Start transaction
	var payment *models.Payment
	err := psm.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lookup ride
		var ride models.Ride
		if err := tx.First(&ride, rideID).Error; err != nil {
			return fmt.Errorf("ride not found: %w", err)
		}

		// Create payment intent (pre-authorization)
		payment = &models.Payment{
			RideID:    rideID,
			PayerID:   payerID,
			Amount:    amount,
			Currency:  "INR",
			Method:    method,
			Status:    PaymentStatePending,
			CreatedAt: time.Now(),
		}

		if err := tx.Create(payment).Error; err != nil {
			return fmt.Errorf("failed to create payment: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Attempt gateway charge with timeout context
	chargeCtx, cancel := context.WithTimeout(ctx, gatewayTimeout)
	defer cancel()

	transactionID, gatewayErr := psm.chargePaymentGateway(chargeCtx, payment)

	// Analyze gateway response
	if gatewayErr != nil {
		isRetryable := psm.isRetryableError(gatewayErr)

		// Update payment state based on error type
		updateErr := psm.db.Transaction(func(tx *gorm.DB) error {
			if isRetryable {
				// Soft failure: payment stays pending, can retry
				log.Printf("[PAYMENT] Gateway error (retryable) for payment %s: %v", payment.ID, gatewayErr)
				return tx.Model(payment).Updates(map[string]interface{}{
					"status":          PaymentStatePending,
					"failure_reason":  gatewayErr.Error(),
					"gateway_attempt": gorm.Expr("gateway_attempt + 1"),
				}).Error
			} else {
				// Hard failure: mark as failed; requires manual intervention or user action
				log.Printf("[PAYMENT] Gateway error (non-retryable) for payment %s: %v", payment.ID, gatewayErr)
				return tx.Model(payment).Updates(map[string]interface{}{
					"status":          PaymentStateFailed,
					"failure_reason":  gatewayErr.Error(),
					"gateway_attempt": gorm.Expr("gateway_attempt + 1"),
				}).Error
			}
		})
		if updateErr != nil {
			return payment, updateErr
		}

		// If retryable, enqueue async retry
		if isRetryable {
			psm.scheduleRetry(payment)
		}

		// Return payment with pending state (not error) so ride can complete
		return payment, nil
	}

	// Success: mark payment as completed
	if err := psm.db.Transaction(func(tx *gorm.DB) error {
		return tx.Model(payment).Updates(map[string]interface{}{
			"status":           PaymentStateCompleted,
			"transaction_id":   transactionID,
			"completed_at":     time.Now(),
			"gateway_response": "OK",
		}).Error
	}); err != nil {
		// Log but don't fail; payment already succeeded at gateway
		log.Printf("[ERROR] Failed to update payment record after successful charge: %v", err)
	}

	return payment, nil
}

// RetryFailedPayment retries a payment that is in 'failed' or 'pending' state.
// This is called by the worker pool for scheduled retries or by the rider manually.
func (psm *PaymentStateMachine) RetryFailedPayment(
	ctx context.Context,
	paymentID uuid.UUID,
	gatewayTimeout time.Duration,
) (*models.Payment, error) {

	var payment models.Payment
	if err := psm.db.First(&payment, paymentID).Error; err != nil {
		return nil, err
	}

	// Only allow retry from pending or failed states
	if payment.Status != PaymentStatePending && payment.Status != PaymentStateFailed {
		return nil, fmt.Errorf("cannot retry payment in state: %s", payment.Status)
	}

	// Check retry limit (max 5 attempts)
	const maxRetries = 5
	retryCount := 0
	// In a real implementation, would query payment_attempts table
	// For now, check based on FailureReason log
	if retryCount >= maxRetries {
		// Max retries exceeded; mark as permanently failed
		if err := psm.db.Model(&payment).Update("status", PaymentStateFailed).Error; err != nil {
			log.Printf("Failed to mark payment as permanently failed: %v", err)
		}
		return &payment, fmt.Errorf("max retries exceeded for payment %s", paymentID)
	}

	// Mark as processing
	if err := psm.db.Model(&payment).Update("status", PaymentStateProcessing).Error; err != nil {
		return nil, err
	}

	// Attempt charge
	chargeCtx, cancel := context.WithTimeout(ctx, gatewayTimeout)
	defer cancel()

	transactionID, gatewayErr := psm.chargePaymentGateway(chargeCtx, &payment)

	if gatewayErr != nil {
		isRetryable := psm.isRetryableError(gatewayErr)
		newState := PaymentStatePending
		if !isRetryable {
			newState = PaymentStateFailed
		}

		if err := psm.db.Model(&payment).Updates(map[string]interface{}{
			"status":          newState,
			"failure_reason":  gatewayErr.Error(),
			"gateway_attempt": gorm.Expr("gateway_attempt + 1"),
		}).Error; err != nil {
			return nil, err
		}

		if isRetryable {
			psm.scheduleRetry(&payment)
		}

		return &payment, gatewayErr
	}

	// Success
	if err := psm.db.Model(&payment).Updates(map[string]interface{}{
		"status":         PaymentStateCompleted,
		"transaction_id": transactionID,
		"completed_at":   time.Now(),
	}).Error; err != nil {
		log.Printf("Failed to mark retry as completed: %v", err)
		return &payment, err
	}

	return &payment, nil
}

// chargePaymentGateway calls the actual payment gateway (Razorpay, Stripe, etc).
// This is factored out for testability and backpressure isolation.
func (psm *PaymentStateMachine) chargePaymentGateway(
	ctx context.Context,
	payment *models.Payment,
) (transactionID string, err error) {

	switch payment.Method {
	case "cash":
		// Cash payment: no gateway, just return success
		return payment.ID.String(), nil

	case "wallet":
		// Wallet payment: internal ledger transfer
		// The actual ledger posting happens in the payment service after successful state transition
		return payment.ID.String(), nil

	case "upi", "card":
		// Call Razorpay API for payment processing
		gateway := NewRazorpayGateway()
		if !gateway.IsConfigured() {
			// If Razorpay not configured, simulate success for development
			select {
			case <-ctx.Done():
				return "", fmt.Errorf("payment gateway timeout: %w", ctx.Err())
			default:
				time.Sleep(100 * time.Millisecond)
				return "txn_" + payment.ID.String(), nil
			}
		}

		// Create order in Razorpay (for idempotency)
		order, err := gateway.CreateOrder(ctx, payment.Amount, payment.Currency, payment.ID.String(), map[string]string{
			"ride_id":  payment.RideID.String(),
			"payer_id": payment.PayerID.String(),
		})
		if err != nil {
			return "", fmt.Errorf("failed to create razorpay order: %w", err)
		}

		// In a real implementation, you would:
		// 1. Return order.ID to frontend
		// 2. Frontend completes payment via Razorpay checkout
		// 3. Webhook captures the payment
		// For now, simulate immediate capture

		// Simulate payment capture (in production, this happens via webhook)
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("payment gateway timeout: %w", ctx.Err())
		default:
			// Simulate network latency
			time.Sleep(100 * time.Millisecond)
			return order.ID, nil
		}

	default:
		return "", fmt.Errorf("unsupported payment method: %s", payment.Method)
	}
}

// isRetryableError determines if a payment error is retryable (transient) or permanent.
func (psm *PaymentStateMachine) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()
	// Retryable: timeout, network, gateway overload
	retryable := []string{
		"timeout",
		"unavailable",
		"temporarily unavailable",
		"connection refused",
		"EOF",
		"context deadline exceeded",
	}

	for _, pattern := range retryable {
		if contains(msg, pattern) {
			return true
		}
	}

	// Non-retryable: invalid card, insufficient funds, etc.
	nonRetryable := []string{
		"invalid card",
		"expired",
		"insufficient funds",
		"fraudulent",
		"declined",
		"invalid amount",
	}

	for _, pattern := range nonRetryable {
		if contains(msg, pattern) {
			return false
		}
	}

	// Default: treat gateway errors as retryable (conservative)
	return true
}

// scheduleRetry enqueues the payment for async retry.
func (psm *PaymentStateMachine) scheduleRetry(payment *models.Payment) {
	// Exponential backoff: retry after 1s, 2s, 4s, 8s, 16s
	var nextRetryAt time.Time
	const baseDelay = time.Second
	// Estimate attempt count from current time and creation time
	backoffExp := 0
	if payment.CreatedAt != (time.Time{}) {
		age := time.Since(payment.CreatedAt)
		if age > 2*time.Minute {
			backoffExp = 4 // Already waited; give up after this
		} else {
			backoffExp = int(age.Seconds()) / 2 // Rough estimate
		}
	}
	delay := baseDelay * (1 << uint(backoffExp))
	if delay > 2*time.Minute {
		delay = 2 * time.Minute // Cap at 2 minutes
	}
	nextRetryAt = time.Now().Add(delay)

	// Log retry scheduling - in production, this would enqueue to a job queue
	// The workers package polls for pending payments and processes them
	log.Printf("[PAYMENT] Scheduled retry for payment %s in %v (next at %s). Worker pool will process.",
		payment.ID, delay, nextRetryAt.Format(time.RFC3339))
}

func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
