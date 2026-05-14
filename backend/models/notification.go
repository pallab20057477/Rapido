package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Notification types
const (
	NotificationTypeRideRequest     = "ride_request"
	NotificationTypeRideAccepted      = "ride_accepted"
	NotificationTypeDriverArrived     = "driver_arrived"
	NotificationTypeRideStarted       = "ride_started"
	NotificationTypeRideCompleted     = "ride_completed"
	NotificationTypePaymentReceived   = "payment_received"
	NotificationTypePromoCode         = "promo_code"
	NotificationTypeDriverVerified    = "driver_verified"
	NotificationTypeSOSAlert          = "sos_alert"
	NotificationTypeSystem            = "system"
	NotificationTypeMarketing         = "marketing"
)

// Notification channels
const (
	NotificationChannelPush = "push"
	NotificationChannelSMS  = "sms"
	NotificationChannelEmail = "email"
	NotificationChannelInApp = "in_app"
)

// Notification status
const (
	NotificationStatusPending   = "pending"
	NotificationStatusSent      = "sent"
	NotificationStatusDelivered = "delivered"
	NotificationStatusRead      = "read"
	NotificationStatusFailed    = "failed"
)

type Notification struct {
	ID        uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;"`
	UserID    uuid.UUID      `json:"user_id" gorm:"not null;index"`
	Type      string         `json:"type" gorm:"not null"`
	Title     string         `json:"title" gorm:"not null"`
	Body      string         `json:"body" gorm:"not null"`
	Data      JSONMap        `json:"data,omitempty" gorm:"type:jsonb"`
	Channels  []string       `json:"channels,omitempty" gorm:"type:text[]"`
	Status    string         `json:"status" gorm:"default:pending"`
	Priority  string         `json:"priority" gorm:"default:normal"` // low, normal, high, urgent
	SentAt    *time.Time     `json:"sent_at,omitempty"`
	ReadAt    *time.Time     `json:"read_at,omitempty"`
	Error     string         `json:"error,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

type NotificationPreference struct {
	ID                  uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;"`
	UserID              uuid.UUID  `json:"user_id" gorm:"uniqueIndex;not null"`
	PushEnabled         bool       `json:"push_enabled" gorm:"default:true"`
	SMSEnabled          bool       `json:"sms_enabled" gorm:"default:true"`
	EmailEnabled        bool       `json:"email_enabled" gorm:"default:true"`
	RideUpdatesPush     bool       `json:"ride_updates_push" gorm:"default:true"`
	RideUpdatesSMS      bool       `json:"ride_updates_sms" gorm:"default:false"`
	PromotionsPush      bool       `json:"promotions_push" gorm:"default:true"`
	PromotionsEmail     bool       `json:"promotions_email" gorm:"default:true"`
	MarketingEmails     bool       `json:"marketing_emails" gorm:"default:false"`
	SafetyAlertsPush    bool       `json:"safety_alerts_push" gorm:"default:true"`
	SafetyAlertsSMS     bool       `json:"safety_alerts_sms" gorm:"default:true"`
	QuietHoursStart     *time.Time `json:"quiet_hours_start,omitempty"`
	QuietHoursEnd       *time.Time `json:"quiet_hours_end,omitempty"`
	Locale              string     `json:"locale" gorm:"default:en"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type DeviceToken struct {
	ID        uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;"`
	UserID    uuid.UUID  `json:"user_id" gorm:"not null;index"`
	Token     string     `json:"token" gorm:"uniqueIndex;not null"`
	Platform  string     `json:"platform" gorm:"not null"` // ios, android, web
	DeviceID  string     `json:"device_id,omitempty"`
	AppVersion string    `json:"app_version,omitempty"`
	IsActive  bool       `json:"is_active" gorm:"default:true"`
	LastUsed  time.Time  `json:"last_used"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// NotificationQueue for background processing
type NotificationQueue struct {
	ID             uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;"`
	NotificationID uuid.UUID  `json:"notification_id" gorm:"not null;index"`
	Channel        string     `json:"channel" gorm:"not null"`
	Status         string     `json:"status" gorm:"default:pending"`
	Attempts       int        `json:"attempts" gorm:"default:0"`
	MaxAttempts    int        `json:"max_attempts" gorm:"default:3"`
	NextRetryAt    *time.Time `json:"next_retry_at,omitempty"`
	ProcessedAt    *time.Time `json:"processed_at,omitempty"`
	Error          string     `json:"error,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (n *Notification) BeforeCreate(tx *gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	return nil
}

func (np *NotificationPreference) BeforeCreate(tx *gorm.DB) error {
	if np.ID == uuid.Nil {
		np.ID = uuid.New()
	}
	return nil
}

func (dt *DeviceToken) BeforeCreate(tx *gorm.DB) error {
	if dt.ID == uuid.Nil {
		dt.ID = uuid.New()
	}
	return nil
}

func (nq *NotificationQueue) BeforeCreate(tx *gorm.DB) error {
	if nq.ID == uuid.Nil {
		nq.ID = uuid.New()
	}
	return nil
}

