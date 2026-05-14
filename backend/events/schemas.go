package events

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// EventSchemaVersion defines the schema version for forward compatibility
type EventSchemaVersion string

const (
	SchemaV1 EventSchemaVersion = "1.0.0"
	SchemaV2 EventSchemaVersion = "2.0.0"
)

// EventType represents all domain events in the system
type EventType string

const (
	// Ride Events
	EventRideRequested       EventType = "ride.requested"
	EventRideMatched           EventType = "ride.matched"
	EventRideAccepted          EventType = "ride.accepted"
	EventRideStarted           EventType = "ride.started"
	EventRideCompleted         EventType = "ride.completed"
	EventRideCancelled         EventType = "ride.cancelled"
	EventRidePaymentCaptured   EventType = "ride.payment_captured"
	EventRidePaymentFailed     EventType = "ride.payment_failed"
	
	// Driver Events
	EventDriverOnline          EventType = "driver.online"
	EventDriverOffline         EventType = "driver.offline"
	EventDriverLocationUpdated EventType = "driver.location_updated"
	EventDriverEarningsPaid    EventType = "driver.earnings_paid"
	
	// User Events
	EventUserRegistered        EventType = "user.registered"
	EventUserLoggedIn          EventType = "user.logged_in"
	EventUserProfileUpdated    EventType = "user.profile_updated"
	
	// Payment Events
	EventPaymentInitiated      EventType = "payment.initiated"
	EventPaymentCaptured       EventType = "payment.captured"
	EventPaymentRefunded       EventType = "payment.refunded"
	EventWalletTopup           EventType = "wallet.topup"
	
	// Notification Events
	EventNotificationSent      EventType = "notification.sent"
	EventNotificationFailed    EventType = "notification.failed"
	
	// Fraud Events
	EventFraudAlertTriggered   EventType = "fraud.alert_triggered"
	EventFraudReviewRequired   EventType = "fraud.review_required"
)

// EventMetadata contains common fields for all events
type EventMetadata struct {
	EventID        uuid.UUID            `json:"event_id"`
	EventType      EventType            `json:"event_type"`
	SchemaVersion  EventSchemaVersion   `json:"schema_version"`
	Timestamp      time.Time            `json:"timestamp"`
	Source         string               `json:"source"`           // Service name
	CorrelationID  uuid.UUID            `json:"correlation_id"`   // For distributed tracing
	CausationID    *uuid.UUID           `json:"causation_id"`     // Previous event ID
	UserID         *uuid.UUID           `json:"user_id"`
	Region         string               `json:"region"`           // AWS region
	RetryCount     int                  `json:"retry_count"`
}

// DomainEvent is the base interface for all events
type DomainEvent interface {
	GetMetadata() EventMetadata
	GetType() EventType
	ToJSON() ([]byte, error)
	Validate() error
}

// baseEvent provides common functionality
type baseEvent struct {
	Metadata EventMetadata `json:"metadata"`
}

func (e baseEvent) GetMetadata() EventMetadata {
	return e.Metadata
}

func (e baseEvent) GetType() EventType {
	return e.Metadata.EventType
}

func (e baseEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// ==================== RIDE EVENTS ====================

// RideRequestedEvent - When rider requests a ride
type RideRequestedEvent struct {
	baseEvent
	Data struct {
		RideID          uuid.UUID `json:"ride_id"`
		RiderID         uuid.UUID `json:"rider_id"`
		PickupLat       float64   `json:"pickup_lat"`
		PickupLng       float64   `json:"pickup_lng"`
		PickupAddress   string    `json:"pickup_address"`
		DropoffLat      float64   `json:"dropoff_lat"`
		DropoffLng      float64   `json:"dropoff_lng"`
		DropoffAddress  string    `json:"dropoff_address"`
		VehicleType     string    `json:"vehicle_type"`
		EstimatedFare   float64   `json:"estimated_fare"`
		SurgeMultiplier float64   `json:"surge_multiplier"`
		Preferences     struct {
			FemaleDriverOnly bool `json:"female_driver_only"`
			ACRequired       bool `json:"ac_required"`
			LuggageSpace     bool `json:"luggage_space"`
		} `json:"preferences"`
	} `json:"data"`
}

func (e RideRequestedEvent) Validate() error {
	if e.Data.RideID == uuid.Nil {
		return fmt.Errorf("ride_id is required")
	}
	if e.Data.RiderID == uuid.Nil {
		return fmt.Errorf("rider_id is required")
	}
	if e.Data.PickupLat == 0 && e.Data.PickupLng == 0 {
		return fmt.Errorf("pickup coordinates are required")
	}
	return nil
}

// NewRideRequestedEvent creates a new ride requested event
func NewRideRequestedEvent(rideID, riderID uuid.UUID, pickupLat, pickupLng float64) RideRequestedEvent {
	return RideRequestedEvent{
		baseEvent: baseEvent{
			Metadata: EventMetadata{
				EventID:       uuid.New(),
				EventType:     EventRideRequested,
				SchemaVersion: SchemaV1,
				Timestamp:     time.Now().UTC(),
				Source:        "ride-service",
				CorrelationID: uuid.New(),
				RetryCount:    0,
			},
		},
		Data: struct {
			RideID          uuid.UUID `json:"ride_id"`
			RiderID         uuid.UUID `json:"rider_id"`
			PickupLat       float64   `json:"pickup_lat"`
			PickupLng       float64   `json:"pickup_lng"`
			PickupAddress   string    `json:"pickup_address"`
			DropoffLat      float64   `json:"dropoff_lat"`
			DropoffLng      float64   `json:"dropoff_lng"`
			DropoffAddress  string    `json:"dropoff_address"`
			VehicleType     string    `json:"vehicle_type"`
			EstimatedFare   float64   `json:"estimated_fare"`
			SurgeMultiplier float64   `json:"surge_multiplier"`
			Preferences     struct {
				FemaleDriverOnly bool `json:"female_driver_only"`
				ACRequired       bool `json:"ac_required"`
				LuggageSpace     bool `json:"luggage_space"`
			} `json:"preferences"`
		}{
			RideID:  rideID,
			RiderID: riderID,
			PickupLat: pickupLat,
			PickupLng: pickupLng,
		},
	}
}

// RideMatchedEvent - When driver is matched
type RideMatchedEvent struct {
	baseEvent
	Data struct {
		RideID       uuid.UUID `json:"ride_id"`
		DriverID     uuid.UUID `json:"driver_id"`
		MatchScore   float64   `json:"match_score"`
		ETASeconds   int       `json:"eta_seconds"`
		DistanceKM   float64   `json:"distance_km"`
		WaveNumber   int       `json:"wave_number"`
		Algorithm    string    `json:"algorithm"` // "legacy" or "ml_v1"
	} `json:"data"`
}

func (e RideMatchedEvent) Validate() error {
	if e.Data.RideID == uuid.Nil || e.Data.DriverID == uuid.Nil {
		return fmt.Errorf("ride_id and driver_id are required")
	}
	return nil
}

// RideCompletedEvent - When ride finishes
type RideCompletedEvent struct {
	baseEvent
	Data struct {
		RideID         uuid.UUID `json:"ride_id"`
		RiderID        uuid.UUID `json:"rider_id"`
		DriverID       uuid.UUID `json:"driver_id"`
		FinalFare      float64   `json:"final_fare"`
		DistanceKM     float64   `json:"distance_km"`
		DurationMin    int       `json:"duration_min"`
		PaymentStatus  string    `json:"payment_status"`
		Rating         *int      `json:"rating,omitempty"`
	} `json:"data"`
}

func (e RideCompletedEvent) Validate() error {
	if e.Data.RideID == uuid.Nil {
		return fmt.Errorf("ride_id is required")
	}
	if e.Data.FinalFare < 0 {
		return fmt.Errorf("final_fare cannot be negative")
	}
	return nil
}

// ==================== PAYMENT EVENTS ====================

// PaymentCapturedEvent - When payment is successfully captured
type PaymentCapturedEvent struct {
	baseEvent
	Data struct {
		PaymentID       uuid.UUID `json:"payment_id"`
		RideID          uuid.UUID `json:"ride_id"`
		UserID          uuid.UUID `json:"user_id"`
		Amount          float64   `json:"amount"`
		Currency        string    `json:"currency"`
		PaymentMethod   string    `json:"payment_method"`
		Gateway         string    `json:"gateway"`
		GatewayRef      string    `json:"gateway_ref"`
		PlatformFee     float64   `json:"platform_fee"`
		DriverEarnings  float64   `json:"driver_earnings"`
	} `json:"data"`
}

func (e PaymentCapturedEvent) Validate() error {
	if e.Data.PaymentID == uuid.Nil {
		return fmt.Errorf("payment_id is required")
	}
	if e.Data.Amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}
	return nil
}

// ==================== FRAUD EVENTS ====================

// FraudAlertEvent - When potential fraud is detected
type FraudAlertEvent struct {
	baseEvent
	Data struct {
		AlertID         uuid.UUID         `json:"alert_id"`
		RideID          *uuid.UUID        `json:"ride_id,omitempty"`
		UserID          uuid.UUID         `json:"user_id"`
		AlertType       string            `json:"alert_type"` // gps_spoofing, account_takeover, etc.
		Severity        string            `json:"severity"`   // low, medium, high, critical
		RiskScore       float64           `json:"risk_score"`
		TriggeredRules  []string          `json:"triggered_rules"`
		Evidence        map[string]interface{} `json:"evidence"`
		ActionTaken     string            `json:"action_taken"` // block, review, allow
	} `json:"data"`
}

func (e FraudAlertEvent) Validate() error {
	if e.Data.AlertID == uuid.Nil || e.Data.UserID == uuid.Nil {
		return fmt.Errorf("alert_id and user_id are required")
	}
	if e.Data.RiskScore < 0 || e.Data.RiskScore > 1 {
		return fmt.Errorf("risk_score must be between 0 and 1")
	}
	return nil
}

// EventRegistry stores all event schemas for validation
type EventRegistry struct {
	schemas map[EventType]SchemaDefinition
}

// SchemaDefinition defines the structure of an event type
type SchemaDefinition struct {
	Version     EventSchemaVersion
	Type        EventType
	RequiredFields []string
	OptionalFields []string
}

// NewEventRegistry creates a registry with all event schemas
func NewEventRegistry() *EventRegistry {
	return &EventRegistry{
		schemas: map[EventType]SchemaDefinition{
			EventRideRequested: {
				Version:        SchemaV1,
				Type:           EventRideRequested,
				RequiredFields: []string{"ride_id", "rider_id", "pickup_lat", "pickup_lng"},
				OptionalFields: []string{"preferences", "surge_multiplier"},
			},
			EventRideMatched: {
				Version:        SchemaV1,
				Type:           EventRideMatched,
				RequiredFields: []string{"ride_id", "driver_id", "match_score"},
				OptionalFields: []string{"eta_seconds", "wave_number"},
			},
			EventPaymentCaptured: {
				Version:        SchemaV1,
				Type:           EventPaymentCaptured,
				RequiredFields: []string{"payment_id", "amount", "currency"},
				OptionalFields: []string{"platform_fee", "driver_earnings"},
			},
			EventFraudAlertTriggered: {
				Version:        SchemaV1,
				Type:           EventFraudAlertTriggered,
				RequiredFields: []string{"alert_id", "user_id", "risk_score"},
				OptionalFields: []string{"ride_id", "evidence"},
			},
		},
	}
}

// ValidateEvent checks if an event conforms to its schema
func (r *EventRegistry) ValidateEvent(eventType EventType, payload []byte) error {
	schema, exists := r.schemas[eventType]
	if !exists {
		return fmt.Errorf("unknown event type: %s", eventType)
	}
	
	// Parse payload
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	
	// Check required fields
	for _, field := range schema.RequiredFields {
		if _, ok := data[field]; !ok {
			return fmt.Errorf("required field missing: %s", field)
		}
	}
	
	return nil
}

// GetSchema returns the schema for an event type
func (r *EventRegistry) GetSchema(eventType EventType) (SchemaDefinition, bool) {
	schema, exists := r.schemas[eventType]
	return schema, exists
}
