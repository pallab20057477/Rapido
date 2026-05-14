package services

import (
	"encoding/json"
	"fmt"
)

// KafkaSchemaRegistry manages Avro/Protobuf schemas for event contracts
type KafkaSchemaRegistry struct {
	schemas map[string]EventSchema
}

// EventSchema defines the structure for Kafka events
type EventSchema struct {
	Name          string                 `json:"name"`
	Version       string                 `json:"version"`
	Format        string                 `json:"format"` // avro, protobuf, json
	Schema        map[string]interface{} `json:"schema"`
	Compatibility string                 `json:"compatibility"` // backward, forward, full
}

// KafkaEvent is the base event envelope
type KafkaEvent struct {
	EventID      string                 `json:"event_id"`
	EventType    string                 `json:"event_type"`
	EventVersion string                 `json:"event_version"`
	Timestamp    string                 `json:"timestamp"`
	Source       string                 `json:"source"`
	Payload      map[string]interface{} `json:"payload"`
	Metadata     KafkaEventMetadata     `json:"metadata"`
}

type KafkaEventMetadata struct {
	CorrelationID string            `json:"correlation_id"`
	UserID        string            `json:"user_id,omitempty"`
	CityID        string            `json:"city_id,omitempty"`
	TraceID       string            `json:"trace_id"`
	Tags          map[string]string `json:"tags,omitempty"`
}

// RideCreatedEvent - ride.created
func GetRideCreatedEventSchema() EventSchema {
	return EventSchema{
		Name:          "ride.created",
		Version:       "1.0.0",
		Format:        "json",
		Compatibility: "backward",
		Schema: map[string]interface{}{
			"type":     "object",
			"required": []string{"event_id", "event_type", "ride_id", "user_id", "timestamp"},
			"properties": map[string]interface{}{
				"event_id":     map[string]string{"type": "string", "format": "uuid"},
				"event_type":   map[string]string{"type": "string", "const": "ride.created"},
				"ride_id":      map[string]string{"type": "string", "format": "uuid"},
				"user_id":      map[string]string{"type": "string", "format": "uuid"},
				"driver_id":    map[string]string{"type": "string", "format": "uuid"},
				"city_id":      map[string]string{"type": "string"},
				"vehicle_type": map[string]string{"type": "string", "enum": "bike,auto,cab"},
				"pickup": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"lat":     map[string]string{"type": "number"},
						"lng":     map[string]string{"type": "number"},
						"address": map[string]string{"type": "string"},
					},
				},
				"drop": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"lat":     map[string]string{"type": "number"},
						"lng":     map[string]string{"type": "number"},
						"address": map[string]string{"type": "string"},
					},
				},
				"estimated_fare": map[string]string{"type": "number"},
				"timestamp":      map[string]string{"type": "string", "format": "date-time"},
			},
		},
	}
}

// RideAcceptedEvent - ride.accepted
func GetRideAcceptedEventSchema() EventSchema {
	return EventSchema{
		Name:    "ride.accepted",
		Version: "1.0.0",
		Format:  "json",
		Schema: map[string]interface{}{
			"type":     "object",
			"required": []string{"ride_id", "driver_id", "accepted_at"},
			"properties": map[string]interface{}{
				"ride_id":     map[string]string{"type": "string", "format": "uuid"},
				"driver_id":   map[string]string{"type": "string", "format": "uuid"},
				"user_id":     map[string]string{"type": "string", "format": "uuid"},
				"accepted_at": map[string]string{"type": "string", "format": "date-time"},
				"driver_location": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"lat": map[string]string{"type": "number"},
						"lng": map[string]string{"type": "number"},
					},
				},
				"eta_seconds": map[string]string{"type": "integer"},
			},
		},
	}
}

// PaymentProcessedEvent - payment.processed
func GetPaymentProcessedEventSchema() EventSchema {
	return EventSchema{
		Name:    "payment.processed",
		Version: "1.0.0",
		Format:  "json",
		Schema: map[string]interface{}{
			"type":     "object",
			"required": []string{"payment_id", "ride_id", "amount", "status"},
			"properties": map[string]interface{}{
				"payment_id":     map[string]string{"type": "string", "format": "uuid"},
				"ride_id":        map[string]string{"type": "string", "format": "uuid"},
				"user_id":        map[string]string{"type": "string", "format": "uuid"},
				"driver_id":      map[string]string{"type": "string", "format": "uuid"},
				"amount":         map[string]string{"type": "number"},
				"currency":       map[string]string{"type": "string", "default": "INR"},
				"method":         map[string]string{"type": "string", "enum": "cash,upi,card,wallet"},
				"status":         map[string]string{"type": "string", "enum": "success,failed,pending"},
				"processed_at":   map[string]string{"type": "string", "format": "date-time"},
				"failure_reason": map[string]string{"type": "string"},
			},
		},
	}
}

// DriverLocationUpdatedEvent - driver.location.updated
func GetDriverLocationUpdatedEventSchema() EventSchema {
	return EventSchema{
		Name:    "driver.location.updated",
		Version: "1.0.0",
		Format:  "json",
		Schema: map[string]interface{}{
			"type":     "object",
			"required": []string{"driver_id", "location", "timestamp"},
			"properties": map[string]interface{}{
				"driver_id": map[string]string{"type": "string", "format": "uuid"},
				"ride_id":   map[string]string{"type": "string", "format": "uuid"},
				"location": map[string]interface{}{
					"type":     "object",
					"required": []string{"lat", "lng"},
					"properties": map[string]interface{}{
						"lat":      map[string]string{"type": "number"},
						"lng":      map[string]string{"type": "number"},
						"accuracy": map[string]string{"type": "number"},
						"heading":  map[string]string{"type": "number"},
						"speed":    map[string]string{"type": "number"},
					},
				},
				"timestamp":     map[string]string{"type": "string", "format": "date-time"},
				"battery_level": map[string]string{"type": "integer"},
			},
		},
	}
}

// RideCompletedEvent - ride.completed (for saga pattern)
func GetRideCompletedEventSchema() EventSchema {
	return EventSchema{
		Name:    "ride.completed",
		Version: "1.0.0",
		Format:  "json",
		Schema: map[string]interface{}{
			"type":     "object",
			"required": []string{"ride_id", "driver_id", "user_id", "final_fare", "distance_km", "duration_min"},
			"properties": map[string]interface{}{
				"ride_id":        map[string]string{"type": "string", "format": "uuid"},
				"driver_id":      map[string]string{"type": "string", "format": "uuid"},
				"user_id":        map[string]string{"type": "string", "format": "uuid"},
				"final_fare":     map[string]string{"type": "number"},
				"distance_km":    map[string]string{"type": "number"},
				"duration_min":   map[string]string{"type": "integer"},
				"completed_at":   map[string]string{"type": "string", "format": "date-time"},
				"payment_status": map[string]string{"type": "string", "enum": "pending,processing,completed"},
			},
		},
	}
}

// EventTopics defines all Kafka topics with schemas
func GetEventTopics() map[string]EventSchema {
	return map[string]EventSchema{
		// Ride Events
		"ride-events":    GetRideCreatedEventSchema(),
		"ride-accepted":  GetRideAcceptedEventSchema(),
		"ride-completed": GetRideCompletedEventSchema(),
		"ride-cancelled": GetRideCreatedEventSchema(), // Same schema with different type

		// Payment Events
		"payment-events":  GetPaymentProcessedEventSchema(),
		"payment-success": GetPaymentProcessedEventSchema(),
		"payment-failed":  GetPaymentProcessedEventSchema(),

		// Driver Events
		"driver-location": GetDriverLocationUpdatedEventSchema(),
		"driver-status":   GetDriverLocationUpdatedEventSchema(),

		// System Events
		"notifications":    {},
		"fraud-events":     {},
		"analytics-events": {},
		"dead-letter":      {},
	}
}

// ValidateEvent validates an event against its schema
func (k *KafkaSchemaRegistry) ValidateEvent(eventType string, payload []byte) error {
	schema, exists := k.schemas[eventType]
	if !exists {
		return fmt.Errorf("schema not found for event type: %s", eventType)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		return fmt.Errorf("invalid JSON payload: %w", err)
	}

	// Basic validation
	if schema.Schema["required"] != nil {
		required := schema.Schema["required"].([]string)
		for _, field := range required {
			if _, exists := data[field]; !exists {
				return fmt.Errorf("required field missing: %s", field)
			}
		}
	}

	return nil
}

// GetSchemaRegistry returns the registry
func GetSchemaRegistry() *KafkaSchemaRegistry {
	return &KafkaSchemaRegistry{
		schemas: GetEventTopics(),
	}
}

// TopicConfig returns topic configuration
func GetTopicConfig() map[string]interface{} {
	return map[string]interface{}{
		"ride-events": map[string]interface{}{
			"partitions":     12,
			"replication":    3,
			"retention":      "7 days",
			"cleanup.policy": "delete",
			"compression":    "snappy",
		},
		"payment-events": map[string]interface{}{
			"partitions":     6,
			"replication":    3,
			"retention":      "30 days",
			"cleanup.policy": "compact",
			"compression":    "snappy",
		},
		"driver-location": map[string]interface{}{
			"partitions":     24,
			"replication":    3,
			"retention":      "1 day",
			"cleanup.policy": "delete",
			"compression":    "lz4",
		},
		"notifications": map[string]interface{}{
			"partitions":     6,
			"replication":    3,
			"retention":      "3 days",
			"cleanup.policy": "delete",
		},
	}
}
