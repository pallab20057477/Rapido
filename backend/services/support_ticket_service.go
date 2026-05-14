package services

import (
	"fmt"
	"log"
	"time"

	"rapido-backend/database"
	"rapido-backend/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SupportTicketService handles support ticket operations
type SupportTicketService struct {
	db *gorm.DB
}

// NewSupportTicketService creates a new service
func NewSupportTicketService() *SupportTicketService {
	return &SupportTicketService{
		db: database.DB,
	}
}

// CreateTicket creates a new support ticket
func (s *SupportTicketService) CreateTicket(userID uuid.UUID, userType string, req models.SupportTicketRequest) (*models.SupportTicket, error) {
	// Generate ticket number
	ticketNumber := generateTicketNumber()

	var rideID *uuid.UUID
	if req.RideID != "" {
		rid, err := uuid.Parse(req.RideID)
		if err != nil {
			return nil, fmt.Errorf("invalid ride_id: must be a valid ride UUID")
		}

		var ride models.Ride
		if err := s.db.First(&ride, rid).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil, fmt.Errorf("ride not found: provide a valid ride_id or omit it")
			}
			return nil, fmt.Errorf("failed to validate ride_id: %w", err)
		}

		rideID = &rid
	}

	ticket := &models.SupportTicket{
		TicketNumber: ticketNumber,
		UserID:       userID,
		UserType:     userType,
		Category:     req.Category,
		Priority:     req.Priority,
		Status:       "open",
		Subject:      req.Subject,
		Description:  req.Description,
		RideID:       rideID,
	}

	if err := s.db.Create(ticket).Error; err != nil {
		return nil, fmt.Errorf("failed to create ticket: %w", err)
	}

	log.Printf("[SupportTicket] Created ticket %s for user %s", ticket.TicketNumber, userID)
	return ticket, nil
}

// GetUserTickets gets all tickets for a user
func (s *SupportTicketService) GetUserTickets(userID uuid.UUID, page, limit int) ([]models.SupportTicket, int64, error) {
	var tickets []models.SupportTicket
	var total int64

	offset := (page - 1) * limit

	if err := s.db.Model(&models.SupportTicket{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := s.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&tickets).Error; err != nil {
		return nil, 0, err
	}

	return tickets, total, nil
}

// GetTicketByID gets a ticket by ID
func (s *SupportTicketService) GetTicketByID(ticketID uuid.UUID) (*models.SupportTicket, []models.SupportTicketMessage, error) {
	var ticket models.SupportTicket
	if err := s.db.First(&ticket, ticketID).Error; err != nil {
		return nil, nil, fmt.Errorf("ticket not found")
	}

	var messages []models.SupportTicketMessage
	if err := s.db.Where("ticket_id = ?", ticketID).Order("created_at ASC").Find(&messages).Error; err != nil {
		return nil, nil, err
	}

	return &ticket, messages, nil
}

// AddMessage adds a message to a ticket
func (s *SupportTicketService) AddMessage(ticketID, senderID uuid.UUID, senderType, message string, isInternal bool) (*models.SupportTicketMessage, error) {
	// Verify ticket exists
	var ticket models.SupportTicket
	if err := s.db.First(&ticket, ticketID).Error; err != nil {
		return nil, fmt.Errorf("ticket not found")
	}

	msg := &models.SupportTicketMessage{
		TicketID:   ticketID,
		SenderID:   senderID,
		SenderType: senderType,
		Message:    message,
		IsInternal: isInternal,
	}

	if err := s.db.Create(msg).Error; err != nil {
		return nil, fmt.Errorf("failed to add message: %w", err)
	}

	// Update ticket updated_at
	s.db.Model(&ticket).Update("updated_at", time.Now())

	return msg, nil
}

// AdminUpdateTicket updates ticket (admin only)
func (s *SupportTicketService) AdminUpdateTicket(ticketID uuid.UUID, adminID uuid.UUID, updates map[string]interface{}) error {
	var ticket models.SupportTicket
	if err := s.db.First(&ticket, ticketID).Error; err != nil {
		return fmt.Errorf("ticket not found")
	}

	// Handle status change
	if status, ok := updates["status"].(string); ok && status == "resolved" {
		now := time.Now()
		updates["resolved_at"] = &now
		updates["resolved_by"] = adminID
	}

	// Handle assignment
	if assignedTo, ok := updates["assigned_to"].(string); ok {
		uid, err := uuid.Parse(assignedTo)
		if err == nil {
			updates["assigned_to"] = uid
		}
	}

	if err := s.db.Model(&ticket).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update ticket: %w", err)
	}

	return nil
}

// AdminGetAllTickets gets all tickets (admin only)
func (s *SupportTicketService) AdminGetAllTickets(status, category string, page, limit int) ([]models.SupportTicket, int64, error) {
	var tickets []models.SupportTicket
	var total int64

	query := s.db.Model(&models.SupportTicket{})

	if status != "" {
		query = query.Where("status = ?", status)
	}
	if category != "" {
		query = query.Where("category = ?", category)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&tickets).Error; err != nil {
		return nil, 0, err
	}

	return tickets, total, nil
}

// generateTicketNumber generates a unique ticket number
func generateTicketNumber() string {
	return fmt.Sprintf("TKT-%d", time.Now().Unix())
}
