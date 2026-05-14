package services

import (
	"fmt"
	"log"
	"time"

	"rapido-backend/database"
	"rapido-backend/models"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// EmergencyContactService handles emergency contact and SOS operations
type EmergencyContactService struct {
	smsService *SMSService
	fcmService *FCMService
}

// NewEmergencyContactService creates a new service
func NewEmergencyContactService() *EmergencyContactService {
	return &EmergencyContactService{
		smsService: GetSMSService(),
		fcmService: GetFCMService(),
	}
}

// AddEmergencyContact adds a new emergency contact for a user
func (s *EmergencyContactService) AddEmergencyContact(userID uuid.UUID, req models.EmergencyContactRequest) (*models.EmergencyContact, error) {
	// Validate userID is not zero/nil
	if userID == uuid.Nil {
		log.Printf("[EmergencyContact] ERROR: userID is nil/zero")
		return nil, fmt.Errorf("invalid user ID: user ID cannot be empty")
	}

	log.Printf("[EmergencyContact] AddContact - userID: %s, Name: %s, Phone: %s", userID.String(), req.Name, req.Phone)

	// Check maximum contacts limit (max 5 per user)
	var count int64
	if err := database.DB.Model(&models.EmergencyContact{}).Where("user_id = ? AND is_active = ?", userID, true).Count(&count).Error; err != nil {
		log.Printf("[EmergencyContact] ERROR: Failed to count contacts - userID: %s, error: %v", userID.String(), err)
		return nil, fmt.Errorf("database error: %w", err)
	}

	if count >= 5 {
		return nil, fmt.Errorf("maximum 5 emergency contacts allowed")
	}

	log.Printf("[EmergencyContact] Validated - userID: %s, current contacts: %d", userID.String(), count)

	contact := &models.EmergencyContact{
		UserID:       userID,
		Name:         req.Name,
		Phone:        req.Phone,
		Relationship: req.Relationship,
		Priority:     req.Priority,
		IsActive:     true,
	}

	log.Printf("[EmergencyContact] Inserting contact - UserID: %s, ContactID: %s", userID.String(), contact.ID)
	if err := database.DB.Create(contact).Error; err != nil {
		log.Printf("[EmergencyContact] ERROR: Failed to create contact - UserID: %s, Error: %v", userID.String(), err)
		// Check if it's a FK constraint error
		errStr := err.Error()
		if strings.Contains(errStr, "violates foreign key constraint") && strings.Contains(errStr, "emergency_contacts") {
			log.Printf("[EmergencyContact] FK Constraint Error: User %s might not exist or was deleted", userID.String())
			return nil, fmt.Errorf("user not found in database (ID: %s) - please re-login", userID.String())
		}
		return nil, fmt.Errorf("failed to create contact: %w", err)
	}

	log.Printf("[EmergencyContact] Added contact %s for user %s", contact.ID, userID)
	return contact, nil
}

// GetEmergencyContacts gets all emergency contacts for a user
func (s *EmergencyContactService) GetEmergencyContacts(userID uuid.UUID) ([]models.EmergencyContact, error) {
	var contacts []models.EmergencyContact
	if err := database.DB.Where("user_id = ? AND is_active = ?", userID, true).
		Order("priority ASC, created_at DESC").
		Find(&contacts).Error; err != nil {
		return nil, fmt.Errorf("failed to get contacts: %w", err)
	}
	return contacts, nil
}

// UpdateEmergencyContact updates an existing contact
func (s *EmergencyContactService) UpdateEmergencyContact(contactID, userID uuid.UUID, req models.EmergencyContactRequest) (*models.EmergencyContact, error) {
	var contact models.EmergencyContact
	if err := database.DB.Where("id = ? AND user_id = ?", contactID, userID).First(&contact).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("contact not found")
		}
		return nil, fmt.Errorf("failed to find contact: %w", err)
	}

	contact.Name = req.Name
	contact.Phone = req.Phone
	contact.Relationship = req.Relationship
	contact.Priority = req.Priority

	if err := database.DB.Save(&contact).Error; err != nil {
		return nil, fmt.Errorf("failed to update contact: %w", err)
	}

	return &contact, nil
}

// RemoveEmergencyContact soft-deletes an emergency contact
func (s *EmergencyContactService) RemoveEmergencyContact(contactID, userID uuid.UUID) error {
	result := database.DB.Where("id = ? AND user_id = ?", contactID, userID).Delete(&models.EmergencyContact{})
	if result.Error != nil {
		return fmt.Errorf("failed to remove contact: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("contact not found")
	}
	return nil
}

// TriggerSOS triggers an emergency SOS alert
func (s *EmergencyContactService) TriggerSOS(userID uuid.UUID, req models.SOSRequest) (*models.SOSResponse, error) {
	// Get user's emergency contacts
	contacts, err := s.GetEmergencyContacts(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get emergency contacts: %w", err)
	}

	if len(contacts) == 0 {
		return nil, fmt.Errorf("no emergency contacts configured")
	}

	// Create SOS event
	var rideID *uuid.UUID
	if req.RideID != "" {
		rid, err := uuid.Parse(req.RideID)
		if err == nil {
			rideID = &rid
		}
	}

	sosEvent := &models.SOSEvent{
		UserID:    userID,
		RideID:    rideID,
		Latitude:  req.Latitude,
		Longitude: req.Longitude,
		Address:   req.Address,
		Status:    "active",
	}

	if err := database.DB.Create(sosEvent).Error; err != nil {
		return nil, fmt.Errorf("failed to create SOS event: %w", err)
	}

	// Notify emergency contacts
	contactsNotified := 0
	for _, contact := range contacts {
		if err := s.notifyEmergencyContact(sosEvent, contact); err != nil {
			log.Printf("[SOS] Failed to notify contact %s: %v", contact.ID, err)
		} else {
			contactsNotified++
		}
	}

	// Also notify via FCM to safety team
	if s.fcmService != nil && s.fcmService.IsEnabled() {
		s.fcmService.SendSOSTriggered(userID, sosEvent.ID.String(), req.Address)
	}

	// Log the SOS event
	log.Printf("[SOS] Event %s triggered by user %s. Contacts notified: %d/%d",
		sosEvent.ID, userID, contactsNotified, len(contacts))

	return &models.SOSResponse{
		EventID:          sosEvent.ID.String(),
		Status:           "active",
		ContactsNotified: contactsNotified,
		Message:          fmt.Sprintf("SOS alert sent to %d emergency contacts", contactsNotified),
	}, nil
}

// notifyEmergencyContact sends SMS notification to an emergency contact
func (s *EmergencyContactService) notifyEmergencyContact(event *models.SOSEvent, contact models.EmergencyContact) error {
	// Build emergency message
	message := fmt.Sprintf("🚨 EMERGENCY ALERT from %s! Location: %s (Lat: %.6f, Lng: %.6f). Time: %s. Please check immediately!",
		contact.Name,
		event.Address,
		event.Latitude,
		event.Longitude,
		event.CreatedAt.Format("15:04:05"))

	// Create notification record
	notification := &models.SOSNotification{
		SOSEventID:       event.ID,
		ContactID:        contact.ID,
		NotificationType: "sms",
		Status:           "pending",
	}

	if err := database.DB.Create(notification).Error; err != nil {
		log.Printf("[SOS] Failed to create notification record: %v", err)
	}

	// Send SMS
	if s.smsService != nil {
		if err := s.smsService.SendSMS(contact.Phone, message); err != nil {
			notification.Status = "failed"
			notification.ErrorMessage = err.Error()
			database.DB.Save(notification)
			return fmt.Errorf("failed to send SMS: %w", err)
		}
	}

	// Update notification status
	now := time.Now()
	notification.Status = "sent"
	notification.SentAt = &now
	if err := database.DB.Save(notification).Error; err != nil {
		log.Printf("[SOS] Failed to update notification status: %v", err)
	}

	return nil
}

// ResolveSOS resolves an active SOS event
func (s *EmergencyContactService) ResolveSOS(eventID uuid.UUID, resolvedBy uuid.UUID, notes string) error {
	var event models.SOSEvent
	if err := database.DB.Where("id = ? AND status = ?", eventID, "active").First(&event).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("SOS event not found or already resolved")
		}
		return fmt.Errorf("failed to find SOS event: %w", err)
	}

	now := time.Now()
	event.Status = "resolved"
	event.ResolvedAt = &now
	event.ResolvedBy = &resolvedBy
	event.Notes = notes

	if err := database.DB.Save(&event).Error; err != nil {
		return fmt.Errorf("failed to resolve SOS event: %w", err)
	}

	log.Printf("[SOS] Event %s resolved by user %s", eventID, resolvedBy)
	return nil
}

// GetActiveSOSEvents gets all active SOS events (for admin)
func (s *EmergencyContactService) GetActiveSOSEvents(page, limit int) ([]models.SOSEvent, int64, error) {
	var events []models.SOSEvent
	var total int64

	offset := (page - 1) * limit

	if err := database.DB.Model(&models.SOSEvent{}).Where("status = ?", "active").Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count events: %w", err)
	}

	if err := database.DB.Where("status = ?", "active").
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&events).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get events: %w", err)
	}

	return events, total, nil
}

// GetUserSOSEvents gets SOS history for a specific user
func (s *EmergencyContactService) GetUserSOSEvents(userID uuid.UUID, page, limit int) ([]models.SOSEvent, int64, error) {
	var events []models.SOSEvent
	var total int64

	offset := (page - 1) * limit

	if err := database.DB.Model(&models.SOSEvent{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count events: %w", err)
	}

	if err := database.DB.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&events).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get events: %w", err)
	}

	return events, total, nil
}

// GetSMSService returns the SMS service instance
func GetSMSService() *SMSService {
	// Return global SMS service instance
	return smsServiceInstance
}

var smsServiceInstance *SMSService
