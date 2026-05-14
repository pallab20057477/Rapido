package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Message types
const (
	MessageTypeText     = "text"
	MessageTypeImage    = "image"
	MessageTypeLocation = "location"
	MessageTypeVoice    = "voice"
	MessageTypeSystem   = "system"
)

// Message status
const (
	MessageStatusSending   = "sending"
	MessageStatusSent      = "sent"
	MessageStatusDelivered = "delivered"
	MessageStatusRead      = "read"
	MessageStatusFailed    = "failed"
)

type ChatRoom struct {
	ID            uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;"`
	RideID        uuid.UUID      `json:"ride_id" gorm:"not null;uniqueIndex"`
	RiderID       uuid.UUID      `json:"rider_id" gorm:"not null"`
	DriverID      uuid.UUID      `json:"driver_id" gorm:"not null"`
	IsActive      bool           `json:"is_active" gorm:"default:true"`
	LastMessageAt *time.Time     `json:"last_message_at,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`
}

type ChatMessage struct {
	ID          uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;"`
	RoomID      uuid.UUID      `json:"room_id" gorm:"not null;index"`
	SenderID    uuid.UUID      `json:"sender_id" gorm:"not null"`
	SenderType  string         `json:"sender_type" gorm:"not null"` // rider, driver, system
	Type        string         `json:"type" gorm:"default:text"`
	Content     string         `json:"content"`
	MediaURL    string         `json:"media_url,omitempty"`
	Latitude    float64        `json:"lat,omitempty"`
	Longitude   float64        `json:"lng,omitempty"`
	Status      string         `json:"status" gorm:"default:sending"`
	SentAt      time.Time      `json:"sent_at"`
	DeliveredAt *time.Time     `json:"delivered_at,omitempty"`
	ReadAt      *time.Time     `json:"read_at,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

type ChatReadReceipt struct {
	ID                uuid.UUID `json:"id" gorm:"type:uuid;primary_key;"`
	RoomID            uuid.UUID `json:"room_id" gorm:"not null;index"`
	UserID            uuid.UUID `json:"user_id" gorm:"not null"`
	LastReadMessageID uuid.UUID `json:"last_read_message_id" gorm:"not null"`
	ReadAt            time.Time `json:"read_at"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// Predefined chat messages
type ChatQuickReply struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key;"`
	Category  string    `json:"category" gorm:"not null"` // rider, driver, both
	Message   string    `json:"message" gorm:"not null"`
	Language  string    `json:"language" gorm:"default:en"`
	Order     int       `json:"order" gorm:"default:0"`
	IsActive  bool      `json:"is_active" gorm:"default:true"`
	CreatedAt time.Time `json:"created_at"`
}

func (cr *ChatRoom) BeforeCreate(tx *gorm.DB) error {
	if cr.ID == uuid.Nil {
		cr.ID = uuid.New()
	}
	return nil
}

func (cm *ChatMessage) BeforeCreate(tx *gorm.DB) error {
	if cm.ID == uuid.Nil {
		cm.ID = uuid.New()
	}
	if cm.SentAt.IsZero() {
		cm.SentAt = time.Now()
	}
	return nil
}

func (crr *ChatReadReceipt) BeforeCreate(tx *gorm.DB) error {
	if crr.ID == uuid.Nil {
		crr.ID = uuid.New()
	}
	return nil
}

func (cqr *ChatQuickReply) BeforeCreate(tx *gorm.DB) error {
	if cqr.ID == uuid.Nil {
		cqr.ID = uuid.New()
	}
	return nil
}
