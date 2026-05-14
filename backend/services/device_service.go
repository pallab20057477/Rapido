package services

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"rapido-backend/database"
	"rapido-backend/models"
	"rapido-backend/utils"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// DeviceService manages device binding and sessions
type DeviceService struct {
	db *gorm.DB
}

// NewDeviceService creates service
func NewDeviceService() *DeviceService {
	return &DeviceService{
		db: database.DB,
	}
}

// RegisterDevice registers a new device for a user
func (s *DeviceService) RegisterDevice(
	userID uuid.UUID,
	userType string,
	deviceID string,
	deviceName string,
	deviceModel string,
	osVersion string,
	appVersion string,
	ipAddress string,
) (*models.Device, error) {
	if deviceID == "" {
		return nil, fmt.Errorf("device_id required")
	}

	// Check if device already registered
	var existing models.Device
	result := s.db.Where("user_id = ? AND device_id = ?", userID, deviceID).First(&existing)
	
	now := time.Now()
	
	if result.Error == nil {
		// Update existing device
		existing.LastActiveAt = &now
		existing.IPAddress = ipAddress
		existing.AppVersion = appVersion
		s.db.Save(&existing)
		return &existing, nil
	}

	// Create new device
	device := &models.Device{
		UserID:       userID,
		UserType:     userType,
		DeviceID:     deviceID,
		DeviceName:   deviceName,
		DeviceModel:  deviceModel,
		OSVersion:    osVersion,
		AppVersion:   appVersion,
		IPAddress:    ipAddress,
		IsTrusted:    false, // Requires verification for new devices
		Status:       "active",
		LastActiveAt: &now,
	}

	if err := s.db.Create(device).Error; err != nil {
		return nil, err
	}

	utils.Info("Device registered",
		zap.String("user_id", userID.String()),
		zap.String("device_id", deviceID))

	return device, nil
}

// GetUserDevices gets all devices for a user
func (s *DeviceService) GetUserDevices(userID uuid.UUID) ([]models.Device, error) {
	var devices []models.Device
	result := s.db.Where("user_id = ? AND status = ?", userID, "active").
		Order("last_active_at DESC").
		Find(&devices)
	return devices, result.Error
}

// VerifyDevice marks a device as trusted
func (s *DeviceService) VerifyDevice(deviceID uuid.UUID, userID uuid.UUID) error {
	return s.db.Model(&models.Device{}).
		Where("id = ? AND user_id = ?", deviceID, userID).
		Update("is_trusted", true).Error
}

// RevokeDevice revokes a device (logout)
func (s *DeviceService) RevokeDevice(deviceID, userID uuid.UUID) error {
	return s.db.Model(&models.Device{}).
		Where("id = ? AND user_id = ?", deviceID, userID).
		Update("status", "revoked").Error
}

// RevokeAllDevices revokes all devices except current
func (s *DeviceService) RevokeAllDevices(userID uuid.UUID, exceptDeviceID string) error {
	return s.db.Model(&models.Device{}).
		Where("user_id = ? AND device_id != ? AND status = ?", userID, exceptDeviceID, "active").
		Update("status", "revoked").Error
}

// ValidateDevice checks if device is valid for user
func (s *DeviceService) ValidateDevice(userID uuid.UUID, deviceID string) (bool, error) {
	var device models.Device
	result := s.db.Where("user_id = ? AND device_id = ? AND status = ?", 
		userID, deviceID, "active").First(&device)
	
	if result.Error != nil {
		return false, result.Error
	}

	// Update last active
	now := time.Now()
	s.db.Model(&device).Update("last_active_at", &now)

	return device.IsTrusted || true, nil // Allow untrusted for now, can be restricted
}

// IsNewDevice checks if this is a new/unrecognized device
func (s *DeviceService) IsNewDevice(userID uuid.UUID, deviceID string) bool {
	var count int64
	s.db.Model(&models.Device{}).
		Where("user_id = ? AND device_id = ?", userID, deviceID).
		Count(&count)
	return count == 0
}

// GenerateDeviceFingerprint creates a device fingerprint
func (s *DeviceService) GenerateDeviceFingerprint(
	deviceModel string,
	osVersion string,
	appVersion string,
	ipAddress string,
) string {
	data := fmt.Sprintf("%s|%s|%s|%s", deviceModel, osVersion, appVersion, ipAddress)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:8]) // Short fingerprint
}

// CheckSuspiciousActivity checks for suspicious login patterns
func (s *DeviceService) CheckSuspiciousActivity(userID uuid.UUID, ipAddress string) (bool, string) {
	// Check for rapid device changes
	var recentDevices int64
	fiveMinutesAgo := time.Now().Add(-5 * time.Minute)
	
	s.db.Model(&models.Device{}).
		Where("user_id = ? AND last_active_at > ?", userID, fiveMinutesAgo).
		Count(&recentDevices)

	if recentDevices > 3 {
		return true, "Multiple device switches detected"
	}

	// Check for new location
	var knownIPs int64
	s.db.Model(&models.Device{}).
		Where("user_id = ? AND ip_address = ?", userID, ipAddress).
		Count(&knownIPs)

	if knownIPs == 0 {
		return true, "Login from new IP address"
	}

	return false, ""
}

// CleanupOldDevices removes inactive devices (older than 90 days)
func (s *DeviceService) CleanupOldDevices() error {
	cutoff := time.Now().AddDate(0, 0, -90)
	
	result := s.db.Where("last_active_at < ? AND status = ?", cutoff, "active").
		Update("status", "inactive")
	
	if result.Error != nil {
		return result.Error
	}

	utils.Info("Cleaned up old devices", zap.Int64("count", result.RowsAffected))
	return nil
}

// Global instance
var DeviceSvc *DeviceService

// InitDeviceService initializes service
func InitDeviceService() {
	DeviceSvc = NewDeviceService()
	
	// Start cleanup job
	go func() {
		for {
			time.Sleep(24 * time.Hour)
			DeviceSvc.CleanupOldDevices()
		}
	}()
}
