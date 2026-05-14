package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RideStatusLog tracks all status changes for a ride (audit trail)
type RideStatusLog struct {
	ID            uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;"`
	RideID        uuid.UUID  `json:"ride_id" gorm:"not null;index"`
	FromStatus    string     `json:"from_status" gorm:"not null"`
	ToStatus      string     `json:"to_status" gorm:"not null"`
	Reason        string     `json:"reason,omitempty"`
	ActorID       *uuid.UUID `json:"actor_id,omitempty" gorm:"index"` // User who triggered the change
	ActorType     string     `json:"actor_type,omitempty"`              // rider, driver, system, admin
	LocationLat   *float64   `json:"location_lat,omitempty"`
	LocationLng   *float64   `json:"location_lng,omitempty"`
	Metadata      JSONMap    `json:"metadata,omitempty" gorm:"type:jsonb"` // Additional context
	CreatedAt     time.Time  `json:"created_at"`
}

// TableName specifies the table name
func (RideStatusLog) TableName() string {
	return "ride_status_logs"
}

// BeforeCreate hook
func (rsl *RideStatusLog) BeforeCreate(tx *gorm.DB) error {
	if rsl.ID == uuid.Nil {
		rsl.ID = uuid.New()
	}
	return nil
}

// RideTimelineEntry represents a timeline entry for API responses
type RideTimelineEntry struct {
	Status     string    `json:"status"`
	Timestamp  time.Time `json:"timestamp"`
	Actor      string    `json:"actor,omitempty"`
	Location   *struct {
		Lat float64 `json:"lat"`
		Lng float64 `json:"lng"`
	} `json:"location,omitempty"`
	Description string `json:"description,omitempty"`
}

// RideTimelineResponse represents the full timeline for a ride
type RideTimelineResponse struct {
	RideID    uuid.UUID             `json:"ride_id"`
	CreatedAt time.Time             `json:"created_at"`
	Entries   []RideTimelineEntry   `json:"entries"`
	CurrentStatus string            `json:"current_status"`
}
