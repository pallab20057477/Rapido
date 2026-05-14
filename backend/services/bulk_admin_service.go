package services

import (
	"fmt"
	"time"

	"rapido-backend/database"
	"rapido-backend/models"
	"rapido-backend/utils"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// BulkAdminService handles bulk admin operations
type BulkAdminService struct {
	db *gorm.DB
}

// NewBulkAdminService creates service
func NewBulkAdminService() *BulkAdminService {
	return &BulkAdminService{
		db: database.DB,
	}
}

// BulkVerifyDrivers verifies multiple drivers at once
func (s *BulkAdminService) BulkVerifyDrivers(
	adminID uuid.UUID,
	driverIDs []uuid.UUID,
	notes string,
) (*BulkOperationResult, error) {
	result := &BulkOperationResult{
		Total:   len(driverIDs),
		Success: 0,
		Failed:  0,
		Errors:  make(map[string]string),
	}

	for _, driverID := range driverIDs {
		driver := &models.Driver{}
		if err := s.db.Where("id = ?", driverID).First(driver).Error; err != nil {
			result.Failed++
			result.Errors[driverID.String()] = "Driver not found"
			continue
		}

		// Update verification status
		err := s.db.Model(driver).Updates(map[string]interface{}{
			"verification_status": "verified",
			"verified_by":         adminID,
			"verified_at":         time.Now(),
			"is_available":        true,
		}).Error

		if err != nil {
			result.Failed++
			result.Errors[driverID.String()] = err.Error()
		} else {
			result.Success++
		}
	}

	utils.Info("Bulk driver verification completed",
		zap.String("admin_id", adminID.String()),
		zap.Int("total", result.Total),
		zap.Int("success", result.Success),
		zap.Int("failed", result.Failed))

	return result, nil
}

// BulkSendNotifications sends notifications to multiple users
func (s *BulkAdminService) BulkSendNotifications(
	adminID uuid.UUID,
	userIDs []uuid.UUID,
	userType string, // "rider" or "driver"
	notification BulkNotification,
) (*BulkOperationResult, error) {
	result := &BulkOperationResult{
		Total:   len(userIDs),
		Success: 0,
		Failed:  0,
		Errors:  make(map[string]string),
	}

	// Queue notifications for background processing
	for _, userID := range userIDs {
		job := map[string]interface{}{
			"type":      "bulk_notification",
			"user_id":   userID,
			"user_type": userType,
			"title":     notification.Title,
			"body":      notification.Body,
			"channels":  notification.Channels,
			"admin_id":  adminID,
			"sent_at":   time.Now(),
		}

		// Add to queue (simplified - in production use Redis queue)
		// QueueJob(job)
		_ = job
		result.Success++
	}

	utils.Info("Bulk notifications queued",
		zap.String("admin_id", adminID.String()),
		zap.Int("count", result.Total))

	return result, nil
}

// BulkUpdateDriverStatus updates status for multiple drivers
func (s *BulkAdminService) BulkUpdateDriverStatus(
	adminID uuid.UUID,
	driverIDs []uuid.UUID,
	status string,
	reason string,
) (*BulkOperationResult, error) {
	result := &BulkOperationResult{
		Total:   len(driverIDs),
		Success: 0,
		Failed:  0,
		Errors:  make(map[string]string),
	}

	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}

	// Validate status
	validStatuses := map[string]bool{
		"active": true, "inactive": true, "suspended": true,
	}
	if !validStatuses[status] {
		return nil, fmt.Errorf("invalid status: %s", status)
	}

	for _, driverID := range driverIDs {
		err := s.db.Model(&models.Driver{}).Where("id = ?", driverID).Updates(updates).Error
		if err != nil {
			result.Failed++
			result.Errors[driverID.String()] = err.Error()
		} else {
			result.Success++
		}
	}

	return result, nil
}

// BulkExportDriverData exports driver data for reporting
func (s *BulkAdminService) BulkExportDriverData(
	driverIDs []uuid.UUID,
) ([]ExportedDriverData, error) {
	var drivers []models.Driver
	result := s.db.Where("id IN ?", driverIDs).Find(&drivers)
	if result.Error != nil {
		return nil, result.Error
	}

	var exported []ExportedDriverData
	for _, driver := range drivers {
		// Get additional stats
		var rideCount int64
		s.db.Model(&models.Ride{}).Where("driver_id = ? AND status = ?", driver.ID, "completed").Count(&rideCount)

		var earnings float64
		s.db.Raw("SELECT COALESCE(SUM(final_fare), 0) FROM rides WHERE driver_id = ? AND status = ?", driver.ID, "completed").Scan(&earnings)

		// Get user info if available
		userName := ""
		userPhone := ""
		userEmail := ""
		if driver.User != nil {
			userName = driver.User.Name
			userPhone = driver.User.Phone
			userEmail = driver.User.Email
		}

		verificationStatus := "unverified"
		if driver.IsVerified {
			verificationStatus = "verified"
		}

		data := ExportedDriverData{
			DriverID:      driver.ID,
			Name:          userName,
			Phone:         userPhone,
			Email:         userEmail,
			Status:        map[bool]string{true: "active", false: "inactive"}[driver.IsActive],
			Verification:  verificationStatus,
			TotalRides:    int(rideCount),
			TotalEarnings: earnings,
			JoinedAt:      driver.CreatedAt,
		}
		exported = append(exported, data)
	}

	return exported, nil
}

// BulkImportDrivers imports drivers from CSV/JSON
func (s *BulkAdminService) BulkImportDrivers(
	adminID uuid.UUID,
	drivers []ImportDriverData,
) (*BulkOperationResult, error) {
	result := &BulkOperationResult{
		Total:   len(drivers),
		Success: 0,
		Failed:  0,
		Errors:  make(map[string]string),
	}

	for i, data := range drivers {
		// Validate required fields
		if data.Phone == "" {
			result.Failed++
			result.Errors[fmt.Sprintf("row_%d", i)] = "Missing phone number"
			continue
		}

		if data.Name == "" {
			result.Failed++
			result.Errors[fmt.Sprintf("row_%d", i)] = "Missing name"
			continue
		}

		// Check if user already exists
		var existingUser models.User
		if err := s.db.Where("phone = ?", data.Phone).First(&existingUser).Error; err == nil {
			// User exists, check if driver already exists
			var existingDriver models.Driver
			if err := s.db.Where("user_id = ?", existingUser.ID).First(&existingDriver).Error; err == nil {
				result.Failed++
				result.Errors[fmt.Sprintf("row_%d", i)] = "Driver already exists for this phone"
				continue
			}

			// Create driver for existing user with minimal required fields
			// License and RC details need to be provided by driver later
			driver := &models.Driver{
				ID:            uuid.New(),
				UserID:        existingUser.ID,
				LicenseNumber: "PENDING_" + uuid.New().String()[:8], // Temporary placeholder
				RCNumber:      "PENDING_" + uuid.New().String()[:8], // Temporary placeholder
				IsVerified:    false,
				IsActive:      true,
			}

			if err := s.db.Create(driver).Error; err != nil {
				result.Failed++
				result.Errors[fmt.Sprintf("row_%d", i)] = "Failed to create driver: " + err.Error()
				continue
			}

			result.Success++
			continue
		}

		// Create new user
		user := &models.User{
			ID:       uuid.New(),
			Name:     data.Name,
			Phone:    data.Phone,
			Email:    data.Email,
			Role:     "driver",
			IsActive: true,
		}

		// Start transaction
		err := s.db.Transaction(func(tx *gorm.DB) error {
			// Create user
			if err := tx.Create(user).Error; err != nil {
				return fmt.Errorf("failed to create user: %w", err)
			}

			// Create driver profile with minimal required fields
			driver := &models.Driver{
				ID:            uuid.New(),
				UserID:        user.ID,
				LicenseNumber: "PENDING_" + uuid.New().String()[:8], // Temporary placeholder
				RCNumber:      "PENDING_" + uuid.New().String()[:8], // Temporary placeholder
				IsVerified:    false,
				IsActive:      true,
			}

			if err := tx.Create(driver).Error; err != nil {
				return fmt.Errorf("failed to create driver: %w", err)
			}

			// Create vehicle if vehicle info provided
			if data.VehicleType != "" && data.VehicleNumber != "" {
				vehicle := &models.Vehicle{
					ID:          uuid.New(),
					DriverID:    driver.ID,
					Type:        data.VehicleType,
					NumberPlate: data.VehicleNumber,
				}
				if err := tx.Create(vehicle).Error; err != nil {
					return fmt.Errorf("failed to create vehicle: %w", err)
				}
			}

			return nil
		})

		if err != nil {
			result.Failed++
			result.Errors[fmt.Sprintf("row_%d", i)] = err.Error()
			continue
		}

		// TODO: Send welcome SMS to new driver
		// smsService := NewSMSService()
		// smsService.SendSMS(data.Phone, fmt.Sprintf("Welcome %s! Your driver account has been created. Please complete your profile in the app.", data.Name))

		result.Success++
	}

	return result, nil
}

// BulkOperationResult tracks bulk operation results
type BulkOperationResult struct {
	Total   int               `json:"total"`
	Success int               `json:"success"`
	Failed  int               `json:"failed"`
	Errors  map[string]string `json:"errors,omitempty"`
}

// BulkNotification represents a bulk notification
type BulkNotification struct {
	Title    string   `json:"title"`
	Body     string   `json:"body"`
	Channels []string `json:"channels"` // push, sms, email
}

// ExportedDriverData for export
type ExportedDriverData struct {
	DriverID      uuid.UUID `json:"driver_id"`
	Name          string    `json:"name"`
	Phone         string    `json:"phone"`
	Email         string    `json:"email"`
	Status        string    `json:"status"`
	Verification  string    `json:"verification"`
	TotalRides    int       `json:"total_rides"`
	TotalEarnings float64   `json:"total_earnings"`
	JoinedAt      time.Time `json:"joined_at"`
}

// ImportDriverData for import
type ImportDriverData struct {
	Name          string `json:"name"`
	Phone         string `json:"phone"`
	Email         string `json:"email,omitempty"`
	City          string `json:"city"`
	VehicleType   string `json:"vehicle_type"`
	VehicleNumber string `json:"vehicle_number"`
}

// Global instance
var BulkAdminSvc *BulkAdminService

// InitBulkAdminService initializes service
func InitBulkAdminService() {
	BulkAdminSvc = NewBulkAdminService()
}
