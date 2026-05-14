package services

import (
	"rapido-backend/database"
	"rapido-backend/models"
	"strconv"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type NotificationService struct {
	DB *gorm.DB
}

func NewNotificationService() *NotificationService {
	return &NotificationService{DB: database.DB}
}

// CreateNotification creates a notification
func (s *NotificationService) CreateNotification(userID uuid.UUID, notificationType, title, body string, data map[string]interface{}) (*models.Notification, error) {
	notification := &models.Notification{
		UserID:   userID,
		Type:     notificationType,
		Title:    title,
		Body:     body,
		Data:     data,
		Channels: []string{"push", "in_app"},
		Status:   models.NotificationStatusPending,
	}

	if err := s.DB.Create(notification).Error; err != nil {
		return nil, err
	}

	// Send push notification via FCM
	if fcmService := GetFCMService(); fcmService != nil && fcmService.IsEnabled() {
		go func() {
			if err := fcmService.SendPushNotification(userID, title, body, data); err != nil {
				// Update notification status on failure
				s.DB.Model(notification).Updates(map[string]interface{}{
					"status": models.NotificationStatusFailed,
					"error":  err.Error(),
				})
			} else {
				// Mark as sent
				now := time.Now()
				s.DB.Model(notification).Updates(map[string]interface{}{
					"status":  models.NotificationStatusSent,
					"sent_at": now,
				})
			}
		}()
	}

	return notification, nil
}

// SendRideRequestNotification sends ride request to driver
func (s *NotificationService) SendRideRequestNotification(driverID uuid.UUID, rideID uuid.UUID, pickup string, eta int) error {
	_, err := s.CreateNotification(
		driverID,
		models.NotificationTypeRideRequest,
		"New Ride Request",
		"Pickup: "+pickup+" (ETA: "+strconv.Itoa(eta)+" min)",
		map[string]interface{}{
			"ride_id": rideID.String(),
			"type":    "ride_request",
		},
	)
	return err
}

// SendDriverAssignedNotification notifies rider of driver assignment
func (s *NotificationService) SendDriverAssignedNotification(riderID, driverID uuid.UUID, rideID uuid.UUID) error {
	// Get driver details
	var driver models.Driver
	s.DB.Preload("User").First(&driver, driverID)

	_, err := s.CreateNotification(
		riderID,
		models.NotificationTypeRideAccepted,
		"Driver Assigned",
		"Your driver "+driver.User.Name+" is on the way",
		map[string]interface{}{
			"ride_id":   rideID.String(),
			"driver_id": driverID.String(),
			"type":      "driver_assigned",
		},
	)
	return err
}

// MarkAsRead marks notification as read
func (s *NotificationService) MarkAsRead(notificationID, userID uuid.UUID) error {
	return s.DB.Model(&models.Notification{}).
		Where("id = ? AND user_id = ?", notificationID, userID).
		Update("status", models.NotificationStatusRead).Error
}

// GetNotifications gets user's notifications
func (s *NotificationService) GetNotifications(userID uuid.UUID, page, perPage int) ([]models.Notification, int64, error) {
	var notifications []models.Notification
	var count int64

	offset := (page - 1) * perPage

	s.DB.Model(&models.Notification{}).Where("user_id = ?", userID).Count(&count)

	if err := s.DB.Where("user_id = ?", userID).
		Order("created_at DESC").
		Offset(offset).Limit(perPage).
		Find(&notifications).Error; err != nil {
		return nil, 0, err
	}

	return notifications, count, nil
}

// RegisterDeviceToken registers a device token for push notifications
func (s *NotificationService) RegisterDeviceToken(userID uuid.UUID, token, platform string) error {
	// Check if token already exists
	var existing models.DeviceToken
	if err := s.DB.Where("token = ?", token).First(&existing).Error; err == nil {
		// Update user ID if different
		existing.UserID = userID
		existing.IsActive = true
		return s.DB.Save(&existing).Error
	}

	deviceToken := &models.DeviceToken{
		UserID:   userID,
		Token:    token,
		Platform: platform,
		IsActive: true,
	}

	return s.DB.Create(deviceToken).Error
}

// UnregisterDeviceToken unregisters a device token
func (s *NotificationService) UnregisterDeviceToken(userID uuid.UUID, token string) error {
	return s.DB.Model(&models.DeviceToken{}).
		Where("user_id = ? AND token = ?", userID, token).
		Update("is_active", false).Error
}
