package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AuditLog represents an audit trail entry
type AuditLog struct {
	ID           uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;"`
	UserID       *uuid.UUID     `json:"user_id,omitempty" gorm:"index"`
	UserType     string         `json:"user_type" gorm:"not null"`         // rider, driver, admin, system
	Action       string         `json:"action" gorm:"not null;index"`      // ride_completed, payment_processed, driver_verified, etc.
	EntityType   string         `json:"entity_type" gorm:"not null;index"` // ride, driver, payment, user
	EntityID     string         `json:"entity_id,omitempty" gorm:"index"`
	OldValues    JSONMap        `json:"old_values,omitempty" gorm:"type:jsonb"`
	NewValues    JSONMap        `json:"new_values,omitempty" gorm:"type:jsonb"`
	IPAddress    string         `json:"ip_address,omitempty"`
	UserAgent    string         `json:"user_agent,omitempty"`
	DeviceID     string         `json:"device_id,omitempty"`
	RequestID    string         `json:"request_id,omitempty"`
	Status       string         `json:"status" gorm:"default:success"` // success, failed, denied
	ErrorMessage string         `json:"error_message,omitempty"`
	Severity     string         `json:"severity" gorm:"default:info"` // info, warning, critical
	CreatedAt    time.Time      `json:"created_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName for AuditLog
func (AuditLog) TableName() string {
	return "audit_logs"
}

// IsCritical checks if log is critical
func (al *AuditLog) IsCritical() bool {
	return al.Severity == "critical"
}

// AuditAction constants
const (
	AuditActionRideRequested    = "ride_requested"
	AuditActionRideAccepted     = "ride_accepted"
	AuditActionRideCompleted    = "ride_completed"
	AuditActionRideCancelled    = "ride_cancelled"
	AuditActionPaymentProcessed = "payment_processed"
	AuditActionPaymentFailed    = "payment_failed"
	AuditActionDriverVerified   = "driver_verified"
	AuditActionDriverSuspended  = "driver_suspended"
	AuditActionUserRegistered   = "user_registered"
	AuditActionUserLogin        = "user_login"
	AuditActionUserLogout       = "user_logout"
	AuditActionPasswordChanged  = "password_changed"
	AuditActionSettingsUpdated  = "settings_updated"
	AuditActionPiiAccessed      = "pii_accessed"
	AuditActionDataExported     = "data_exported"
	AuditActionRefundProcessed  = "refund_processed"
)

// EntityType constants
const (
	AuditEntityRide    = "ride"
	AuditEntityDriver  = "driver"
	AuditEntityRider   = "rider"
	AuditEntityPayment = "payment"
	AuditEntityUser    = "user"
	AuditEntityAdmin   = "admin"
	AuditEntitySystem  = "system"
)
