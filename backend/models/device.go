package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Device represents a registered user device
type Device struct {
	ID           uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;"`
	UserID       uuid.UUID      `json:"user_id" gorm:"not null;index"`
	UserType     string         `json:"user_type" gorm:"not null"`       // rider, driver
	DeviceID     string         `json:"device_id" gorm:"not null;index"` // Unique device identifier
	DeviceName   string         `json:"device_name"`
	DeviceModel  string         `json:"device_model"`
	OSVersion    string         `json:"os_version"`
	AppVersion   string         `json:"app_version"`
	IPAddress    string         `json:"ip_address"`
	IsTrusted    bool           `json:"is_trusted" gorm:"default:false"`
	Status       string         `json:"status" gorm:"default:active"` // active, revoked, inactive
	LastActiveAt *time.Time     `json:"last_active_at"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName for Device
func (Device) TableName() string {
	return "devices"
}

// IsActive checks if device is active
func (d *Device) IsActive() bool {
	return d.Status == "active"
}

// DeviceSession represents an active session on a device
type DeviceSession struct {
	ID           uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;"`
	DeviceID     uuid.UUID      `json:"device_id" gorm:"not null;index"`
	UserID       uuid.UUID      `json:"user_id" gorm:"not null;index"`
	TokenHash    string         `json:"-" gorm:"not null"`
	IsActive     bool           `json:"is_active" gorm:"default:true"`
	LastActivity time.Time      `json:"last_activity"`
	CreatedAt    time.Time      `json:"created_at"`
	ExpiresAt    time.Time      `json:"expires_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName for DeviceSession
func (DeviceSession) TableName() string {
	return "device_sessions"
}

// IsExpired checks if session has expired
func (ds *DeviceSession) IsExpired() bool {
	return time.Now().After(ds.ExpiresAt)
}
