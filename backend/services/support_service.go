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

// SupportService handles support tickets and disputes
type SupportService struct {
	db *gorm.DB
}

// NewSupportService creates service
func NewSupportService() *SupportService {
	return &SupportService{
		db: database.DB,
	}
}

// CreateTicket creates a new support ticket
func (s *SupportService) CreateTicket(
	userID uuid.UUID,
	userType string,
	category string,
	priority string,
	subject string,
	description string,
	rideID *uuid.UUID,
) (*models.SupportTicket, error) {
	// Generate ticket number (TKT-YYYYMMDD-XXXX)
	ticketNumber := s.generateTicketNumber()

	ticket := &models.SupportTicket{
		TicketNumber: ticketNumber,
		UserID:       userID,
		UserType:     userType,
		Category:     category,
		Priority:     priority,
		Status:       "open",
		Subject:      subject,
		Description:  description,
		RideID:       rideID,
	}

	if err := s.db.Create(ticket).Error; err != nil {
		return nil, err
	}

	utils.Info("Support ticket created",
		zap.String("ticket_number", ticketNumber),
		zap.String("user_id", userID.String()))

	return ticket, nil
}

// GetUserTickets gets tickets for a user
func (s *SupportService) GetUserTickets(userID uuid.UUID, userType string, page, limit int) ([]models.SupportTicket, int64, error) {
	var tickets []models.SupportTicket
	var total int64

	offset := (page - 1) * limit

	s.db.Model(&models.SupportTicket{}).Where("user_id = ? AND user_type = ?", userID, userType).Count(&total)

	result := s.db.Where("user_id = ? AND user_type = ?", userID, userType).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&tickets)

	return tickets, total, result.Error
}

// GetTicket gets a ticket by ID
func (s *SupportService) GetTicket(ticketID uuid.UUID) (*models.SupportTicket, error) {
	var ticket models.SupportTicket
	result := s.db.Preload("Ride").Where("id = ?", ticketID).First(&ticket)
	return &ticket, result.Error
}

// AddMessage adds a message to a ticket
func (s *SupportService) AddMessage(
	ticketID, senderID uuid.UUID,
	senderType string,
	message string,
	isInternal bool,
) (*models.SupportTicketMessage, error) {
	msg := &models.SupportTicketMessage{
		TicketID:   ticketID,
		SenderID:   senderID,
		SenderType: senderType,
		Message:    message,
		IsInternal: isInternal,
	}

	if err := s.db.Create(msg).Error; err != nil {
		return nil, err
	}

	// Update ticket updated_at
	s.db.Model(&models.SupportTicket{}).Where("id = ?", ticketID).Update("updated_at", time.Now())

	return msg, nil
}

// GetTicketMessages gets all messages for a ticket
func (s *SupportService) GetTicketMessages(ticketID uuid.UUID, isAdmin bool) ([]models.SupportTicketMessage, error) {
	var messages []models.SupportTicketMessage

	query := s.db.Where("ticket_id = ?", ticketID)

	// Non-admins can't see internal messages
	if !isAdmin {
		query = query.Where("is_internal = ?", false)
	}

	result := query.Order("created_at ASC").Find(&messages)
	return messages, result.Error
}

// UpdateTicketStatus updates ticket status
func (s *SupportService) UpdateTicketStatus(ticketID uuid.UUID, status string, assignedTo *uuid.UUID) error {
	updates := map[string]interface{}{
		"status": status,
	}

	if assignedTo != nil {
		updates["assigned_to"] = *assignedTo
	}

	if status == "resolved" || status == "closed" {
		now := time.Now()
		updates["resolved_at"] = &now
	}

	return s.db.Model(&models.SupportTicket{}).Where("id = ?", ticketID).Updates(updates).Error
}

// CreateDispute creates a fare/ride dispute
func (s *SupportService) CreateDispute(
	rideID, disputedBy uuid.UUID,
	disputedByType string,
	reason string,
	description string,
	expectedFare, actualFare float64,
) (*models.Dispute, error) {
	// Check if dispute already exists
	var existing models.Dispute
	if err := s.db.Where("ride_id = ?", rideID).First(&existing).Error; err == nil {
		return nil, fmt.Errorf("dispute already exists for this ride")
	}

	dispute := &models.Dispute{
		RideID:         rideID,
		DisputedBy:     disputedBy,
		DisputedByType: disputedByType,
		Reason:         reason,
		Description:    description,
		ExpectedFare:   expectedFare,
		ActualFare:     actualFare,
		Status:         "pending",
	}

	if err := s.db.Create(dispute).Error; err != nil {
		return nil, err
	}

	utils.Info("Dispute created",
		zap.String("ride_id", rideID.String()),
		zap.String("reason", reason))

	return dispute, nil
}

// GetDispute gets a dispute by ID
func (s *SupportService) GetDispute(disputeID uuid.UUID) (*models.Dispute, error) {
	var dispute models.Dispute
	result := s.db.Preload("Ride").Where("id = ?", disputeID).First(&dispute)
	return &dispute, result.Error
}

// ResolveDispute resolves a dispute with decision
func (s *SupportService) ResolveDispute(
	disputeID, resolvedBy uuid.UUID,
	accepted bool,
	refundAmount *float64,
	resolution string,
) error {
	status := "resolved_rejected"
	if accepted {
		status = "resolved_accepted"
	}

	updates := map[string]interface{}{
		"status":       status,
		"resolution":   resolution,
		"resolved_by":  resolvedBy,
		"resolved_at":  time.Now(),
	}

	if refundAmount != nil {
		updates["refund_amount"] = *refundAmount
	}

	return s.db.Model(&models.Dispute{}).Where("id = ?", disputeID).Updates(updates).Error
}

// GetPendingTickets gets tickets needing attention
func (s *SupportService) GetPendingTickets(category string, priority string, page, limit int) ([]models.SupportTicket, int64, error) {
	var tickets []models.SupportTicket
	var total int64

	offset := (page - 1) * limit

	query := s.db.Model(&models.SupportTicket{}).Where("status IN ?", []string{"open", "in_progress"})

	if category != "" {
		query = query.Where("category = ?", category)
	}

	if priority != "" {
		query = query.Where("priority = ?", priority)
	}

	query.Count(&total)

	result := query.Order("priority DESC, created_at ASC").
		Limit(limit).
		Offset(offset).
		Find(&tickets)

	return tickets, total, result.Error
}

// generateTicketNumber generates unique ticket number
func (s *SupportService) generateTicketNumber() string {
	date := time.Now().Format("20060102")
	random := uuid.New().String()[:4]
	return fmt.Sprintf("TKT-%s-%s", date, random)
}

// GetTicketStats gets ticket statistics for admin
func (s *SupportService) GetTicketStats() map[string]interface{} {
	var stats struct {
		Open        int64
		InProgress  int64
		Resolved    int64
		Closed      int64
		Total       int64
	}

	s.db.Model(&models.SupportTicket{}).Where("status = ?", "open").Count(&stats.Open)
	s.db.Model(&models.SupportTicket{}).Where("status = ?", "in_progress").Count(&stats.InProgress)
	s.db.Model(&models.SupportTicket{}).Where("status = ?", "resolved").Count(&stats.Resolved)
	s.db.Model(&models.SupportTicket{}).Where("status = ?", "closed").Count(&stats.Closed)
	s.db.Model(&models.SupportTicket{}).Count(&stats.Total)

	return map[string]interface{}{
		"open":        stats.Open,
		"in_progress": stats.InProgress,
		"resolved":    stats.Resolved,
		"closed":      stats.Closed,
		"total":       stats.Total,
	}
}

// Global instance
var SupportSvc *SupportService

// InitSupportService initializes service
func InitSupportService() {
	SupportSvc = NewSupportService()
}
