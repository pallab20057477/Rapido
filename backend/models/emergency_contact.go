package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// EmergencyContact represents an emergency contact for a user
type EmergencyContact struct {
	ID           uuid.UUID       `json:"id" gorm:"type:uuid;primary_key;"`
	UserID       uuid.UUID       `json:"user_id" gorm:"type:uuid;not null;index"`
	Name         string          `json:"name" gorm:"not null"`
	Phone        string          `json:"phone" gorm:"not null"`
	Relationship string          `json:"relationship"`              // spouse, parent, sibling, friend, etc.
	Priority     int             `json:"priority" gorm:"default:1"` // 1 = primary, 2 = secondary, etc.
	IsActive     bool            `json:"is_active" gorm:"default:true"`
	CreatedAt    time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt    *gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName specifies the table name for EmergencyContact
func (EmergencyContact) TableName() string {
	return "emergency_contacts"
}

// SOSEvent represents an SOS alert triggered by a user
type SOSEvent struct {
	ID         uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;"`
	UserID     uuid.UUID  `json:"user_id" gorm:"type:uuid;not null;index"`
	RideID     *uuid.UUID `json:"ride_id" gorm:"type:uuid;index"`
	Latitude   float64    `json:"latitude" gorm:"not null"`
	Longitude  float64    `json:"longitude" gorm:"not null"`
	Address    string     `json:"address"`
	Status     string     `json:"status" gorm:"default:active"` // active, resolved, false_alarm
	ResolvedAt *time.Time `json:"resolved_at"`
	ResolvedBy *uuid.UUID `json:"resolved_by" gorm:"type:uuid"`
	Notes      string     `json:"notes"`
	CreatedAt  time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt  time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName specifies the table name for SOSEvent
func (SOSEvent) TableName() string {
	return "sos_events"
}

// SOSNotification tracks notifications sent for an SOS event
type SOSNotification struct {
	ID               uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;"`
	SOSEventID       uuid.UUID  `json:"sos_event_id" gorm:"type:uuid;not null;index"`
	ContactID        uuid.UUID  `json:"contact_id" gorm:"type:uuid;not null"`
	NotificationType string     `json:"notification_type" gorm:"not null"` // sms, push, call
	Status           string     `json:"status" gorm:"default:pending"`   // pending, sent, failed, delivered
	SentAt           *time.Time `json:"sent_at"`
	ErrorMessage     string     `json:"error_message"`
	CreatedAt        time.Time  `json:"created_at" gorm:"autoCreateTime"`
}

// TableName specifies the table name for SOSNotification
func (SOSNotification) TableName() string {
	return "sos_notifications"
}

// EmergencyContactRequest represents a request to add/update emergency contact
type EmergencyContactRequest struct {
	Name         string `json:"name" binding:"required"`
	Phone        string `json:"phone" binding:"required"`
	Relationship string `json:"relationship"`
	Priority     int    `json:"priority"`
}

// SOSRequest represents an SOS trigger request
type SOSRequest struct {
	Latitude  float64 `json:"latitude" binding:"required"`
	Longitude float64 `json:"longitude" binding:"required"`
	Address   string  `json:"address"`
	RideID    string  `json:"ride_id"`
}

// SOSResponse represents the response after triggering SOS
type SOSResponse struct {
	EventID          string `json:"event_id"`
	Status           string `json:"status"`
	ContactsNotified int    `json:"contacts_notified"`
	Message          string `json:"message"`
}

