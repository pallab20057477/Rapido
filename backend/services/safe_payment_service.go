package services

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"rapido-backend/database"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Payment states
type PaymentStatus string

const (
	PaymentStatusPending  PaymentStatus = "PENDING"
	PaymentStatusHeld     PaymentStatus = "HELD"
	PaymentStatusSuccess  PaymentStatus = "SUCCESS"
	PaymentStatusFailed   PaymentStatus = "FAILED"
	PaymentStatusRefunded PaymentStatus = "REFUNDED"
)

type SafePaymentService struct {
	db             *gorm.DB
	redis          *redis.Client
	idempotencyTTL time.Duration
}

type PaymentRequest struct {
	RideID         uuid.UUID
	UserID         uuid.UUID
	Amount         float64
	Method         string
	IdempotencyKey string
}

type Payment struct {
	ID             uuid.UUID `gorm:"type:uuid;primary_key"`
	RideID         uuid.UUID `gorm:"type:uuid;index"`
	UserID         uuid.UUID `gorm:"type:uuid;index"`
	Amount         float64
	Status         PaymentStatus
	IdempotencyKey string `gorm:"uniqueIndex"`
	GatewayRef     string
	Attempts       int
	ErrorMessage   string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	ProcessedAt    *time.Time
}

func NewSafePaymentService() *SafePaymentService {
	return &SafePaymentService{
		db:             database.DB,
		redis:          database.RedisClient,
		idempotencyTTL: 24 * time.Hour,
	}
}

func (s *SafePaymentService) ProcessPayment(ctx context.Context, req PaymentRequest) (*Payment, error) {
	// 1. Check idempotency - return existing if already processed
	if existing := s.getIdempotentResult(req.IdempotencyKey); existing != nil {
		return existing, nil
	}

	// 2. Double-charge prevention
	if err := s.checkDoubleCharge(req.UserID, req.Amount); err != nil {
		return nil, err
	}

	// 3. Acquire distributed lock
	lockKey := fmt.Sprintf("payment_lock:%s", req.IdempotencyKey)
	acquired, err := s.redis.SetNX(ctx, lockKey, "1", 30*time.Second).Result()
	if err != nil || !acquired {
		return nil, fmt.Errorf("payment already processing")
	}
	defer s.redis.Del(ctx, lockKey)

	// 4. Start transaction
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 5. Create payment record
	payment := &Payment{
		ID:             uuid.New(),
		RideID:         req.RideID,
		UserID:         req.UserID,
		Amount:         req.Amount,
		Status:         PaymentStatusPending,
		IdempotencyKey: req.IdempotencyKey,
		Attempts:       0,
		CreatedAt:      time.Now(),
	}

	if err := tx.Create(payment).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// 6. Hold amount (pre-authorization)
	if err := s.holdAmount(tx, payment); err != nil {
		payment.Status = PaymentStatusFailed
		payment.ErrorMessage = err.Error()
		tx.Save(payment)
		tx.Commit()
		return payment, err
	}

	payment.Status = PaymentStatusHeld
	now := time.Now()
	payment.ProcessedAt = &now

	// 7. Store idempotency result
	s.storeIdempotentResult(req.IdempotencyKey, payment)

	// 8. Record recent payment for double-spend detection
	s.recordRecentPayment(req.UserID, req.Amount)

	// 9. Commit
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return payment, nil
}

func (s *SafePaymentService) CapturePayment(paymentID uuid.UUID) error {
	var payment Payment
	if err := s.db.First(&payment, "id = ?", paymentID).Error; err != nil {
		return err
	}

	if payment.Status != PaymentStatusHeld {
		return fmt.Errorf("payment not in held state: %s", payment.Status)
	}

	// Capture the held amount (actual charge)
	// In real implementation: call payment gateway capture

	payment.Status = PaymentStatusSuccess
	return s.db.Save(&payment).Error
}

func (s *SafePaymentService) RefundPayment(paymentID uuid.UUID) error {
	var payment Payment
	if err := s.db.First(&payment, "id = ?", paymentID).Error; err != nil {
		return err
	}

	if payment.Status != PaymentStatusSuccess && payment.Status != PaymentStatusHeld {
		return fmt.Errorf("cannot refund payment in state: %s", payment.Status)
	}

	// Process refund via gateway
	// In real implementation: call payment gateway refund

	payment.Status = PaymentStatusRefunded
	return s.db.Save(&payment).Error
}

func (s *SafePaymentService) getIdempotentResult(key string) *Payment {
	var payment Payment
	err := s.db.Where("idempotency_key = ?", key).First(&payment).Error
	if err != nil {
		return nil
	}
	return &payment
}

func (s *SafePaymentService) storeIdempotentResult(key string, payment *Payment) {
	data, _ := json.Marshal(payment)
	s.redis.Set(context.Background(), fmt.Sprintf("idempotency:%s", key), data, s.idempotencyTTL)
}

func (s *SafePaymentService) holdAmount(tx *gorm.DB, payment *Payment) error {
	// Simulate payment gateway hold
	// In production: integrate with Razorpay/Stripe hold API
	payment.GatewayRef = fmt.Sprintf("hold_%s", payment.ID)
	return nil
}

func (s *SafePaymentService) checkDoubleCharge(userID uuid.UUID, amount float64) error {
	key := fmt.Sprintf("recent_payments:%s", userID)

	// Get recent payments from Redis sorted set
	minScore := time.Now().Add(-2 * time.Minute).Unix()
	payments := s.redis.ZRangeByScore(context.Background(), key, &redis.ZRangeBy{
		Min: fmt.Sprintf("%d", minScore),
		Max: fmt.Sprintf("%d", time.Now().Unix()),
	}).Val()

	// Check for similar amounts within 1% tolerance
	for _, payment := range payments {
		parts := strings.Split(payment, ":")
		if len(parts) == 2 {
			pAmount, _ := strconv.ParseFloat(parts[1], 64)
			// Within 1% difference = potential double charge
			if math.Abs(pAmount-amount)/amount < 0.01 {
				return fmt.Errorf("potential double charge detected: similar amount %.2f", pAmount)
			}
		}
	}

	return nil
}

func (s *SafePaymentService) recordRecentPayment(userID uuid.UUID, amount float64) {
	key := fmt.Sprintf("recent_payments:%s", userID)
	s.redis.ZAdd(context.Background(), key, redis.Z{
		Score:  float64(time.Now().Unix()),
		Member: fmt.Sprintf("%s:%.2f", uuid.New(), amount),
	})
	s.redis.Expire(context.Background(), key, 2*time.Minute)
}

func (s *SafePaymentService) HandleWebhook(eventType, paymentRef string) error {
	// Check if webhook already processed
	processedKey := fmt.Sprintf("webhook:%s", paymentRef)
	if s.redis.Exists(context.Background(), processedKey).Val() > 0 {
		return nil // Already processed
	}

	// Find payment by gateway reference
	var payment Payment
	if err := s.db.Where("gateway_ref = ?", paymentRef).First(&payment).Error; err != nil {
		return err
	}

	// Process based on event type
	switch eventType {
	case "payment.captured":
		payment.Status = PaymentStatusSuccess
	case "payment.failed":
		payment.Status = PaymentStatusFailed
	case "payment.refunded":
		payment.Status = PaymentStatusRefunded
	}

	if err := s.db.Save(&payment).Error; err != nil {
		return err
	}

	// Mark as processed
	s.redis.Set(context.Background(), processedKey, "1", 24*time.Hour)
	return nil
}

var SafePaymentServiceInstance *SafePaymentService

func InitSafePaymentService() {
	SafePaymentServiceInstance = NewSafePaymentService()
}
