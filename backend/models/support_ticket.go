package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SupportTicket represents a customer support ticket
type SupportTicket struct {
	ID           uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;"`
	TicketNumber string         `json:"ticket_number" gorm:"uniqueIndex;not null"`
	UserID       uuid.UUID      `json:"user_id" gorm:"not null;index"`
	UserType     string         `json:"user_type" gorm:"not null"`        // rider, driver
	Category     string         `json:"category" gorm:"not null"`         // payment_issue, ride_issue, safety, account, other
	Priority     string         `json:"priority" gorm:"default:medium"` // low, medium, high, critical
	Status       string         `json:"status" gorm:"default:open"`     // open, in_progress, resolved, closed, escalated
	Subject      string         `json:"subject" gorm:"not null"`
	Description  string         `json:"description" gorm:"not null"`
	RideID       *uuid.UUID     `json:"ride_id,omitempty"`
	Ride         *Ride          `json:"ride,omitempty" gorm:"foreignKey:RideID"`
	RefundAmount *float64       `json:"refund_amount,omitempty"`
	RefundStatus string         `json:"refund_status,omitempty"` // pending, processed, rejected
	AssignedTo   *uuid.UUID     `json:"assigned_to,omitempty"`   // Admin ID
	Resolution   string         `json:"resolution,omitempty"`
	ResolvedAt   *time.Time     `json:"resolved_at,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

// SupportTicketMessage represents messages in a ticket thread
type SupportTicketMessage struct {
	ID         uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;"`
	TicketID   uuid.UUID      `json:"ticket_id" gorm:"not null;index"`
	SenderID   uuid.UUID      `json:"sender_id" gorm:"not null"`
	SenderType string         `json:"sender_type" gorm:"not null"` // user, admin, system
	Message    string         `json:"message" gorm:"not null"`
	IsInternal bool           `json:"is_internal" gorm:"default:false"` // Internal admin notes
	CreatedAt  time.Time      `json:"created_at"`
	DeletedAt  gorm.DeletedAt `json:"-" gorm:"index"`
}

// Dispute represents a ride fare/quality dispute
type Dispute struct {
	ID             uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;"`
	RideID         uuid.UUID      `json:"ride_id" gorm:"uniqueIndex;not null"`
	Ride           *Ride          `json:"ride,omitempty" gorm:"foreignKey:RideID"`
	DisputedBy     uuid.UUID      `json:"disputed_by" gorm:"not null"`      // User ID
	DisputedByType string         `json:"disputed_by_type" gorm:"not null"` // rider, driver
	Reason         string         `json:"reason" gorm:"not null"`           // route_manipulation, overcharge, behavior, other
	Description    string         `json:"description" gorm:"not null"`
	ExpectedFare   float64        `json:"expected_fare"`
	ActualFare     float64        `json:"actual_fare"`
	Status         string         `json:"status" gorm:"default:pending"` // pending, under_review, resolved_rejected, resolved_accepted
	AdminNotes     string         `json:"admin_notes,omitempty"`
	RefundAmount   *float64       `json:"refund_amount,omitempty"`
	Resolution     string         `json:"resolution,omitempty"`
	ResolvedBy     *uuid.UUID     `json:"resolved_by,omitempty"`
	ResolvedAt     *time.Time     `json:"resolved_at,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName for SupportTicket
func (SupportTicket) TableName() string {
	return "support_tickets"
}

// TableName for SupportTicketMessage
func (SupportTicketMessage) TableName() string {
	return "support_ticket_messages"
}

// TableName for Dispute
func (Dispute) TableName() string {
	return "disputes"
}

// IsOpen checks if ticket is open
func (st *SupportTicket) IsOpen() bool {
	return st.Status == "open" || st.Status == "in_progress"
}

// CanDispute checks if dispute is allowed
func (d *Dispute) CanDispute() bool {
	return d.Status == "pending" || d.Status == "under_review"
}

// SupportTicketRequest represents a request to create a support ticket
type SupportTicketRequest struct {
	Category    string `json:"category" binding:"required"`
	Priority    string `json:"priority"`
	Subject     string `json:"subject" binding:"required"`
	Description string `json:"description" binding:"required"`
	RideID      string `json:"ride_id"`
}

