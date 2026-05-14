package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"rapido-backend/database"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// EventBus provides reliable event publishing with retry and DLQ
type EventBus struct {
	redis      *redis.Client
	registry   *EventRegistry
	ctx        context.Context
	
	// Retry configuration
	MaxRetries      int
	InitialBackoff  time.Duration
	MaxBackoff      time.Duration
	BackoffMultiplier float64
}

// PublishResult indicates the outcome of a publish operation
type PublishResult struct {
	Success   bool
	EventID   uuid.UUID
	Error     error
	Attempt   int
	Duration  time.Duration
}

// ConsumerHandler is the function signature for event handlers
type ConsumerHandler func(ctx context.Context, event DomainEvent) error

// Consumer represents an event consumer
type Consumer struct {
	Name     string
	Group    string
	Handler  ConsumerHandler
	Filter   func(EventType) bool // Optional filter
}

// NewEventBus creates a new event bus with retry policies
func NewEventBus() *EventBus {
	return &EventBus{
		redis:             database.RedisClient,
		registry:          NewEventRegistry(),
		ctx:               context.Background(),
		MaxRetries:        5,
		InitialBackoff:    1 * time.Second,
		MaxBackoff:        60 * time.Second,
		BackoffMultiplier: 2.0,
	}
}

// Publish publishes an event to the bus with exactly-once semantics
func (eb *EventBus) Publish(event DomainEvent) (*PublishResult, error) {
	start := time.Now()
	
	// Validate event
	if err := event.Validate(); err != nil {
		return &PublishResult{
			Success: false,
			Error:   fmt.Errorf("validation failed: %w", err),
		}, nil
	}
	
	// Serialize event
	payload, err := event.ToJSON()
	if err != nil {
		return &PublishResult{
			Success: false,
			Error:   fmt.Errorf("serialization failed: %w", err),
		}, nil
	}
	
	// Get stream name based on event type
	streamName := eb.getStreamName(event.GetType())
	
	// Publish to Redis Stream
	values := map[string]interface{}{
		"event_type": string(event.GetType()),
		"schema_version": string(event.GetMetadata().SchemaVersion),
		"payload": string(payload),
		"correlation_id": event.GetMetadata().CorrelationID.String(),
		"timestamp": event.GetMetadata().Timestamp.Format(time.RFC3339),
		"retry_count": 0,
	}
	
	// Use Redis transaction for atomicity
	pipe := eb.redis.Pipeline()
	pipe.XAdd(eb.ctx, &redis.XAddArgs{
		Stream: streamName,
		Values: values,
	})
	
	// Also publish to Pub/Sub for real-time consumers
	pubsubMsg := map[string]interface{}{
		"stream": streamName,
		"event_type": string(event.GetType()),
		"event_id": event.GetMetadata().EventID.String(),
	}
	pubsubJSON, _ := json.Marshal(pubsubMsg)
	pipe.Publish(eb.ctx, "event_bus:notifications", pubsubJSON)
	
	_, err = pipe.Exec(eb.ctx)
	if err != nil {
		return &PublishResult{
			Success: false,
			Error:   fmt.Errorf("redis publish failed: %w", err),
		}, nil
	}
	
	return &PublishResult{
		Success:  true,
		EventID:  event.GetMetadata().EventID,
		Duration: time.Since(start),
	}, nil
}

// PublishWithRetry publishes with automatic retry on failure
func (eb *EventBus) PublishWithRetry(event DomainEvent) (*PublishResult, error) {
	var lastErr error
	backoff := eb.InitialBackoff
	
	for attempt := 1; attempt <= eb.MaxRetries; attempt++ {
		result, err := eb.Publish(event)
		if err == nil && result.Success {
			result.Attempt = attempt
			return result, nil
		}
		
		lastErr = err
		if result != nil && result.Error != nil {
			lastErr = result.Error
		}
		
		// Don't retry validation errors
		if err != nil {
			break
		}
		
		// Exponential backoff with jitter
		if attempt < eb.MaxRetries {
			log.Printf("[EventBus] Publish failed (attempt %d/%d), retrying in %v: %v", 
				attempt, eb.MaxRetries, backoff, lastErr)
			time.Sleep(backoff)
			backoff = time.Duration(float64(backoff) * eb.BackoffMultiplier)
			if backoff > eb.MaxBackoff {
				backoff = eb.MaxBackoff
			}
		}
	}
	
	// Max retries exceeded - send to DLQ
	eb.sendToDLQ(event, lastErr, eb.MaxRetries)
	
	return &PublishResult{
		Success: false,
		Error:   fmt.Errorf("max retries exceeded: %w", lastErr),
		Attempt: eb.MaxRetries,
	}, nil
}

// Consume starts consuming events from a stream
func (eb *EventBus) Consume(consumer Consumer) error {
	streamName := eb.getStreamNameForConsumer(consumer)
	groupName := consumer.Group
	
	// Create consumer group if not exists
	eb.redis.XGroupCreateMkStream(eb.ctx, streamName, groupName, "0")
	
	for {
		// Read from stream
		streams, err := eb.redis.XReadGroup(eb.ctx, &redis.XReadGroupArgs{
			Group:    groupName,
			Consumer: consumer.Name,
			Streams:  []string{streamName, ">"},
			Count:    10,
			Block:    5 * time.Second,
		}).Result()
		
		if err != nil {
			if err == redis.Nil {
				continue // No new messages
			}
			log.Printf("[EventBus] Consumer error: %v", err)
			continue
		}
		
		// Process messages
		for _, stream := range streams {
			for _, msg := range stream.Messages {
				eb.processMessage(consumer, stream.Stream, msg)
			}
		}
	}
}

// processMessage processes a single message with retry logic
func (eb *EventBus) processMessage(consumer Consumer, stream string, msg redis.XMessage) {
	ctx := context.Background()
	
	// Extract message data
	eventTypeStr, _ := msg.Values["event_type"].(string)
	payloadStr, _ := msg.Values["payload"].(string)
	retryCount, _ := msg.Values["retry_count"].(int64)
	
	// Parse event
	event, err := eb.parseEvent(EventType(eventTypeStr), []byte(payloadStr))
	if err != nil {
		log.Printf("[EventBus] Failed to parse event: %v", err)
		eb.acknowledgeMessage(stream, consumer.Group, msg.ID)
		return
	}
	
	// Call handler with retry
	backoff := eb.InitialBackoff
	for attempt := 0; attempt <= eb.MaxRetries; attempt++ {
		err = consumer.Handler(ctx, event)
		if err == nil {
			// Success - acknowledge
			eb.acknowledgeMessage(stream, consumer.Group, msg.ID)
			return
		}
		
		// Handler failed
		if attempt < eb.MaxRetries {
			log.Printf("[EventBus] Handler failed (attempt %d/%d), retrying: %v", 
				attempt+1, eb.MaxRetries, err)
			time.Sleep(backoff)
			backoff = time.Duration(float64(backoff) * eb.BackoffMultiplier)
			if backoff > eb.MaxBackoff {
				backoff = eb.MaxBackoff
			}
		}
	}
	
	// Max retries exceeded - send to DLQ
	eb.sendToDLQ(event, err, int(retryCount)+eb.MaxRetries)
	
	// Acknowledge to prevent reprocessing
	eb.acknowledgeMessage(stream, consumer.Group, msg.ID)
}

// parseEvent parses JSON payload into appropriate event type
func (eb *EventBus) parseEvent(eventType EventType, payload []byte) (DomainEvent, error) {
	switch eventType {
	case EventRideRequested:
		var event RideRequestedEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, err
		}
		return event, nil
		
	case EventRideMatched:
		var event RideMatchedEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, err
		}
		return event, nil
		
	case EventPaymentCaptured:
		var event PaymentCapturedEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, err
		}
		return event, nil
		
	case EventFraudAlertTriggered:
		var event FraudAlertEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, err
		}
		return event, nil
		
	default:
		return nil, fmt.Errorf("unknown event type: %s", eventType)
	}
}

// acknowledgeMessage acknowledges a message in a stream
func (eb *EventBus) acknowledgeMessage(stream, group, messageID string) {
	eb.redis.XAck(eb.ctx, stream, group, messageID)
}

// sendToDLQ sends failed events to Dead Letter Queue
func (eb *EventBus) sendToDLQ(event DomainEvent, err error, totalRetries int) {
	dlqEntry := map[string]interface{}{
		"original_event": event,
		"error":          err.Error(),
		"failed_at":      time.Now().UTC(),
		"total_retries":  totalRetries,
		"event_type":     string(event.GetType()),
	}
	
	payload, _ := json.Marshal(dlqEntry)
	
	// Add to DLQ stream
	eb.redis.XAdd(eb.ctx, &redis.XAddArgs{
		Stream: "dlq:events",
		Values: map[string]interface{}{
			"payload": string(payload),
			"timestamp": time.Now().Format(time.RFC3339),
		},
	})
	
	zap.L().Error("[EventBus] Event sent to DLQ",
		zap.String("event_type", string(event.GetType())),
		zap.String("event_id", event.GetMetadata().EventID.String()),
		zap.Error(err),
		zap.Int("retries", totalRetries))
}

// getStreamName returns the Redis stream name for an event type
func (eb *EventBus) getStreamName(eventType EventType) string {
	return fmt.Sprintf("events:%s", eventType)
}

// getStreamNameForConsumer returns stream name for a consumer
func (eb *EventBus) getStreamNameForConsumer(consumer Consumer) string {
	// Default to all events stream
	return "events:all"
}

// GetDLQEvents retrieves events from DLQ for manual review
func (eb *EventBus) GetDLQEvents(count int64) ([]map[string]interface{}, error) {
	messages, err := eb.redis.XRange(eb.ctx, "dlq:events", "-", "+").Result()
	if err != nil {
		return nil, err
	}
	
	var events []map[string]interface{}
	for _, msg := range messages {
		payload, _ := msg.Values["payload"].(string)
		var event map[string]interface{}
		json.Unmarshal([]byte(payload), &event)
		events = append(events, event)
	}
	
	return events, nil
}

// ReplayDLQEvent replays a specific event from DLQ
func (eb *EventBus) ReplayDLQEvent(messageID string) error {
	// Get the event from DLQ
	messages, err := eb.redis.XRange(eb.ctx, "dlq:events", messageID, messageID).Result()
	if err != nil || len(messages) == 0 {
		return fmt.Errorf("event not found in DLQ")
	}
	
	// Parse and republish
	payload, _ := messages[0].Values["payload"].(string)
	var dlqEntry map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &dlqEntry); err != nil {
		return fmt.Errorf("failed to parse DLQ entry: %w", err)
	}
	
	// Remove from DLQ
	eb.redis.XDel(eb.ctx, "dlq:events", messageID)
	
	log.Printf("[EventBus] Replayed event from DLQ: %s", messageID)
	return nil
}

// Global instance
var EventBusInstance *EventBus

// InitEventBus initializes the global event bus
func InitEventBus() {
	EventBusInstance = NewEventBus()
	log.Println("[EventBus] Initialized")
}
