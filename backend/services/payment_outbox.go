package services

import (
	"encoding/json"
	"fmt"
	"time"

	"rapido-backend/database"
	"rapido-backend/utils"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// PaymentOutboxService implements the outbox pattern for payment consistency
type PaymentOutboxService struct {
	db *gorm.DB
}

// OutboxEvent represents a payment event to be processed
type OutboxEvent struct {
	ID            uuid.UUID       `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	AggregateType string          `json:"aggregate_type"` // "payment", "wallet"
	AggregateID   string          `json:"aggregate_id"`   // ride_id, user_id
	EventType     string          `json:"event_type"`     // "payment_initiated", "payment_completed"
	Payload       json.RawMessage `json:"payload"`        // Event data
	Status        string          `json:"status"`         // "pending", "processing", "completed", "failed"
	RetryCount    int             `json:"retry_count"`
	CreatedAt     time.Time       `json:"created_at"`
	ProcessedAt   *time.Time      `json:"processed_at"`
	Error         string          `json:"error"`
}

// NewPaymentOutboxService creates a new outbox service
func NewPaymentOutboxService() *PaymentOutboxService {
	return &PaymentOutboxService{
		db: database.DB,
	}
}

// CreatePaymentEvent creates a new payment event in outbox (within transaction)
func (s *PaymentOutboxService) CreatePaymentEvent(tx *gorm.DB, rideID, userID uuid.UUID, amount float64, method string) (*OutboxEvent, error) {
	payload := map[string]interface{}{
		"ride_id":         rideID.String(),
		"user_id":         userID.String(),
		"amount":          amount,
		"method":          method,
		"currency":        "INR",
		"idempotency_key": uuid.New().String(),
	}

	payloadBytes, _ := json.Marshal(payload)

	event := &OutboxEvent{
		AggregateType: "payment",
		AggregateID:   rideID.String(),
		EventType:     "payment_initiated",
		Payload:       payloadBytes,
		Status:        "pending",
		RetryCount:    0,
		CreatedAt:     time.Now(),
	}

	if err := tx.Create(event).Error; err != nil {
		return nil, fmt.Errorf("failed to create outbox event: %w", err)
	}

	utils.Info("Payment outbox event created",
		zap.String("event_id", event.ID.String()),
		zap.String("ride_id", rideID.String()),
		zap.Float64("amount", amount))

	return event, nil
}

// CreateWalletEvent creates a wallet transaction event
func (s *PaymentOutboxService) CreateWalletEvent(tx *gorm.DB, userID uuid.UUID, amount float64, txnType string) (*OutboxEvent, error) {
	payload := map[string]interface{}{
		"user_id": userID.String(),
		"amount":  amount,
		"type":    txnType,
	}

	payloadBytes, _ := json.Marshal(payload)

	event := &OutboxEvent{
		AggregateType: "wallet",
		AggregateID:   userID.String(),
		EventType:     fmt.Sprintf("wallet_%s", txnType),
		Payload:       payloadBytes,
		Status:        "pending",
		RetryCount:    0,
		CreatedAt:     time.Now(),
	}

	if err := tx.Create(event).Error; err != nil {
		return nil, err
	}

	return event, nil
}

// ProcessPendingEvents processes all pending outbox events
func (s *PaymentOutboxService) ProcessPendingEvents() {
	for {
		events, err := s.fetchPendingEvents(10)
		if err != nil {
			utils.Error("Failed to fetch pending events", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}

		if len(events) == 0 {
			time.Sleep(1 * time.Second)
			continue
		}

		for _, event := range events {
			s.processEvent(&event)
		}
	}
}

// fetchPendingEvents retrieves pending events for processing
func (s *PaymentOutboxService) fetchPendingEvents(limit int) ([]OutboxEvent, error) {
	var events []OutboxEvent

	result := s.db.Where("status = ?", "pending").
		Order("created_at ASC").
		Limit(limit).
		Find(&events)

	return events, result.Error
}

// processEvent processes a single outbox event
func (s *PaymentOutboxService) processEvent(event *OutboxEvent) {
	// Mark as processing
	s.db.Model(event).Update("status", "processing")

	var err error

	switch event.AggregateType {
	case "payment":
		err = s.processPaymentEvent(event)
	case "wallet":
		err = s.processWalletEvent(event)
	}

	now := time.Now()

	if err != nil {
		event.RetryCount++
		event.Error = err.Error()

		if event.RetryCount >= 5 {
			event.Status = "failed"
			utils.Error("Outbox event permanently failed",
				zap.String("event_id", event.ID.String()),
				zap.Error(err))
		} else {
			event.Status = "pending" // Retry later
		}
	} else {
		event.Status = "completed"
		event.ProcessedAt = &now
		event.Error = ""
	}

	s.db.Save(event)
}

// processPaymentEvent processes payment events
func (s *PaymentOutboxService) processPaymentEvent(event *OutboxEvent) error {
	var payload map[string]interface{}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	rideID, _ := uuid.Parse(payload["ride_id"].(string))
	method := payload["method"].(string)

	// amount is for logging only
	_ = payload["amount"].(float64)

	// Process payment through gateway
	paymentService := NewPaymentService()
	idempotencyKey := event.ID.String()
	payment, err := paymentService.ProcessPayment(rideID, method, idempotencyKey)
	if err != nil {
		return fmt.Errorf("payment processing failed: %w", err)
	}

	utils.Info("Payment processed from outbox",
		zap.String("event_id", event.ID.String()),
		zap.String("payment_id", payment.ID.String()))

	return nil
}

// processWalletEvent processes wallet events (placeholder - wallet service not implemented)
func (s *PaymentOutboxService) processWalletEvent(event *OutboxEvent) error {
	// Mark as completed to prevent reprocessing
	// Wallet service integration pending
	utils.Info("Wallet event skipped - service not implemented",
		zap.String("event_id", event.ID.String()))
	return nil
}

// StartOutboxProcessor starts the outbox processing loop
func (s *PaymentOutboxService) StartOutboxProcessor() {
	go s.ProcessPendingEvents()
	utils.Info("Payment outbox processor started")
}

// CleanupOldEvents removes completed/failed events older than retention period
func (s *PaymentOutboxService) CleanupOldEvents(retentionDays int) error {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	result := s.db.Where("(status = ? OR status = ?) AND created_at < ?",
		"completed", "failed", cutoff).
		Delete(&OutboxEvent{})

	if result.Error != nil {
		return result.Error
	}

	utils.Info("Cleaned up old outbox events",
		zap.Int64("deleted", result.RowsAffected))

	return nil
}

// Global instance
var OutboxService *PaymentOutboxService

// InitOutboxService initializes the outbox service
func InitOutboxService() {
	OutboxService = NewPaymentOutboxService()
	OutboxService.StartOutboxProcessor()
}
