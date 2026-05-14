package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"rapido-backend/database"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Event represents a domain event in the system
type Event struct {
	ID        string                 `json:"event_id"`
	Type      string                 `json:"event_type"`
	Timestamp time.Time              `json:"timestamp"`
	Payload   map[string]interface{} `json:"payload"`
	Metadata  EventMetadata          `json:"metadata"`
}

// EventMetadata contains tracing and service info
type EventMetadata struct {
	TraceID  string `json:"trace_id"`
	Service  string `json:"service"`
	Version  string `json:"version"`
	UserID   string `json:"user_id,omitempty"`
	SourceIP string `json:"source_ip,omitempty"`
}

// EventHandler is a function that processes events
type EventHandler func(event Event) error

// EventBus provides event-driven architecture capabilities
type EventBus struct {
	redisClient *redis.Client
	ctx         context.Context
	handlers    map[string][]EventHandler
}

// NewEventBus creates a new event bus instance
func NewEventBus() *EventBus {
	return &EventBus{
		redisClient: database.RedisClient,
		ctx:         context.Background(),
		handlers:    make(map[string][]EventHandler),
	}
}

// Domain event types
const (
	EventRideRequested           = "ride.requested"
	EventRideAccepted            = "ride.accepted"
	EventRideStarted             = "ride.started"
	EventRideCompleted           = "ride.completed"
	EventRideCancelled           = "ride.cancelled"
	EventRideNoDriverFound       = "ride.no_driver_found"
	EventPaymentCompleted        = "payment.completed"
	EventPaymentFailed           = "payment.failed"
	EventDriverLocationUpdated   = "driver.location_updated"
	EventFraudDetected           = "fraud.detected"
	EventSupportTicketCreated    = "support.ticket.created"
	EventDriverIncentiveAchieved = "driver.incentive.achieved"
)

// Publish emits an event to the event bus
func (eb *EventBus) Publish(eventType string, payload map[string]interface{}, metadata *EventMetadata) error {
	event := Event{
		ID:        uuid.New().String(),
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		Payload:   payload,
		Metadata: EventMetadata{
			TraceID: generateTraceID(),
			Service: "rapido-backend",
			Version: "v4.0.0",
		},
	}

	if metadata != nil {
		event.Metadata = *metadata
	}

	// Serialize event
	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Publish to Redis Stream
	streamKey := fmt.Sprintf("events:%s", eventType)
	_, err = eb.redisClient.XAdd(eb.ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: map[string]interface{}{
			"data": string(eventData),
		},
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to publish event to stream: %w", err)
	}

	// Also publish to Pub/Sub for real-time consumers
	pubsubChannel := fmt.Sprintf("events:%s:pubsub", eventType)
	eb.redisClient.Publish(eb.ctx, pubsubChannel, eventData)

	return nil
}

// Subscribe registers a handler for an event type
func (eb *EventBus) Subscribe(eventType string, handler EventHandler) {
	eb.handlers[eventType] = append(eb.handlers[eventType], handler)
}

// StartConsumer starts consuming events from Redis Streams
func (eb *EventBus) StartConsumer(eventType string, consumerGroup string) error {
	streamKey := fmt.Sprintf("events:%s", eventType)

	// Create consumer group if it doesn't exist
	err := eb.redisClient.XGroupCreateMkStream(eb.ctx, streamKey, consumerGroup, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return fmt.Errorf("failed to create consumer group: %w", err)
	}

	// Start consuming
	go eb.consumeEvents(streamKey, consumerGroup, eventType)

	return nil
}

// consumeEvents reads events from the stream and dispatches to handlers
func (eb *EventBus) consumeEvents(streamKey, consumerGroup, eventType string) {
	consumerID := uuid.New().String()

	for {
		// Read from stream
		streams, err := eb.redisClient.XReadGroup(eb.ctx, &redis.XReadGroupArgs{
			Group:    consumerGroup,
			Consumer: consumerID,
			Streams:  []string{streamKey, ">"},
			Count:    10,
			Block:    5 * time.Second,
		}).Result()

		if err != nil {
			if err != redis.Nil {
				// Log error but continue
				fmt.Printf("Error reading from stream %s: %v\n", streamKey, err)
			}
			continue
		}

		// Process messages
		for _, stream := range streams {
			for _, message := range stream.Messages {
				eb.processMessage(streamKey, consumerGroup, message, eventType)
			}
		}
	}
}

// processMessage handles a single message
func (eb *EventBus) processMessage(streamKey, consumerGroup string, message redis.XMessage, eventType string) {
	// Extract event data
	data, ok := message.Values["data"].(string)
	if !ok {
		// Acknowledge invalid message
		eb.redisClient.XAck(eb.ctx, streamKey, consumerGroup, message.ID)
		return
	}

	// Parse event
	var event Event
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		// Acknowledge invalid message
		eb.redisClient.XAck(eb.ctx, streamKey, consumerGroup, message.ID)
		return
	}

	// Dispatch to handlers
	handlers := eb.handlers[eventType]
	success := true

	for _, handler := range handlers {
		if err := handler(event); err != nil {
			// Log error but continue with other handlers
			fmt.Printf("Handler error for event %s: %v\n", eventType, err)
			success = false
		}
	}

	// Acknowledge message if all handlers succeeded
	if success {
		eb.redisClient.XAck(eb.ctx, streamKey, consumerGroup, message.ID)
	}
}

// PublishRideRequested emits a ride requested event
func (eb *EventBus) PublishRideRequested(rideID, riderID uuid.UUID, pickupLat, pickupLng float64, vehicleType string) error {
	return eb.Publish(EventRideRequested, map[string]interface{}{
		"ride_id":      rideID.String(),
		"rider_id":     riderID.String(),
		"pickup_lat":   pickupLat,
		"pickup_lng":   pickupLng,
		"vehicle_type": vehicleType,
		"requested_at": time.Now().UTC(),
	}, nil)
}

// PublishRideAccepted emits a ride accepted event
func (eb *EventBus) PublishRideAccepted(rideID, driverID, riderID uuid.UUID, fare float64) error {
	return eb.Publish(EventRideAccepted, map[string]interface{}{
		"ride_id":        rideID.String(),
		"driver_id":      driverID.String(),
		"rider_id":       riderID.String(),
		"estimated_fare": fare,
		"accepted_at":    time.Now().UTC(),
	}, nil)
}

// PublishRideCompleted emits a ride completed event
func (eb *EventBus) PublishRideCompleted(rideID, driverID, riderID uuid.UUID, finalFare float64) error {
	return eb.Publish(EventRideCompleted, map[string]interface{}{
		"ride_id":      rideID.String(),
		"driver_id":    driverID.String(),
		"rider_id":     riderID.String(),
		"final_fare":   finalFare,
		"completed_at": time.Now().UTC(),
	}, nil)
}

// PublishPaymentCompleted emits a payment completed event
func (eb *EventBus) PublishPaymentCompleted(rideID uuid.UUID, amount float64, paymentMethod string) error {
	return eb.Publish(EventPaymentCompleted, map[string]interface{}{
		"ride_id":        rideID.String(),
		"amount":         amount,
		"payment_method": paymentMethod,
		"paid_at":        time.Now().UTC(),
	}, nil)
}

// PublishDriverLocationUpdated emits a driver location update
func (eb *EventBus) PublishDriverLocationUpdated(driverID uuid.UUID, lat, lng float64) error {
	return eb.Publish(EventDriverLocationUpdated, map[string]interface{}{
		"driver_id": driverID.String(),
		"lat":       lat,
		"lng":       lng,
		"timestamp": time.Now().UTC(),
	}, nil)
}

// generateTraceID generates a unique trace ID
func generateTraceID() string {
	return fmt.Sprintf("trace_%s", uuid.New().String()[:8])
}

// Global event bus instance
var EventBusInstance *EventBus

// InitEventBus initializes the global event bus
func InitEventBus() {
	EventBusInstance = NewEventBus()
}
