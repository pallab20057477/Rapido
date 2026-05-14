package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ScheduledRide represents a pre-scheduled ride
type ScheduledRide struct {
	ID                 uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;"`
	RiderID            uuid.UUID      `json:"rider_id" gorm:"not null;index"`
	PickupLat          float64        `json:"pickup_lat" gorm:"not null"`
	PickupLng          float64        `json:"pickup_lng" gorm:"not null"`
	PickupAddress      string         `json:"pickup_address"`
	DropoffLat         float64        `json:"dropoff_lat" gorm:"not null"`
	DropoffLng         float64        `json:"dropoff_lng" gorm:"not null"`
	DropoffAddress     string         `json:"dropoff_address"`
	VehicleType        string         `json:"vehicle_type" gorm:"not null"`
	ScheduledAt        time.Time      `json:"scheduled_at" gorm:"not null;index"`
	Status             string         `json:"status" gorm:"default:pending"` // pending, notified, assigned, completed, cancelled
	Notes              string         `json:"notes"`
	RideID             *uuid.UUID     `json:"ride_id,omitempty"` // Link to actual ride when created
	NotificationSentAt *time.Time     `json:"notification_sent_at,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName specifies the table name
func (ScheduledRide) TableName() string {
	return "scheduled_rides"
}

// IsUpcoming checks if ride is in the future
func (sr *ScheduledRide) IsUpcoming() bool {
	return sr.ScheduledAt.After(time.Now()) && sr.Status != "cancelled"
}

// CanCancel checks if ride can be cancelled
func (sr *ScheduledRide) CanCancel() bool {
	// Can cancel up to 2 hours before scheduled time
	return time.Until(sr.ScheduledAt) > 2*time.Hour
}

// ScheduledRideResponse for API responses
type ScheduledRideResponse struct {
	ID             string    `json:"id"`
	PickupLat      float64   `json:"pickup_lat"`
	PickupLng      float64   `json:"pickup_lng"`
	PickupAddress  string    `json:"pickup_address"`
	DropoffLat     float64   `json:"dropoff_lat"`
	DropoffLng     float64   `json:"dropoff_lng"`
	DropoffAddress string    `json:"dropoff_address"`
	VehicleType    string    `json:"vehicle_type"`
	ScheduledAt    time.Time `json:"scheduled_at"`
	Status         string    `json:"status"`
	Notes          string    `json:"notes"`
	CanCancel      bool      `json:"can_cancel"`
}

// ToResponse converts to API response
func (sr *ScheduledRide) ToResponse() ScheduledRideResponse {
	return ScheduledRideResponse{
		ID:             sr.ID.String(),
		PickupLat:      sr.PickupLat,
		PickupLng:      sr.PickupLng,
		PickupAddress:  sr.PickupAddress,
		DropoffLat:     sr.DropoffLat,
		DropoffLng:     sr.DropoffLng,
		DropoffAddress: sr.DropoffAddress,
		VehicleType:    sr.VehicleType,
		ScheduledAt:    sr.ScheduledAt,
		Status:         sr.Status,
		Notes:          sr.Notes,
		CanCancel:      sr.CanCancel(),
	}
}

