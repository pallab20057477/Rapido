package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SOS Alert status
const (
	SOSStatusActive     = "active"
	SOSStatusResolved   = "resolved"
	SOSStatusFalseAlarm = "false_alarm"
	SOSStatusEscalated  = "escalated"
)

type SOSAlert struct {
	ID              uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;"`
	RideID          *uuid.UUID     `json:"ride_id,omitempty" gorm:"index"`
	UserID          uuid.UUID      `json:"user_id" gorm:"not null;index"`
	UserType        string         `json:"user_type" gorm:"not null"` // rider, driver
	Status          string         `json:"status" gorm:"default:active"`
	Latitude        float64        `json:"lat" gorm:"not null"`
	Longitude       float64        `json:"lng" gorm:"not null"`
	Address         string         `json:"address,omitempty"`
	Reason          string         `json:"reason,omitempty"`
	TriggeredBy     string         `json:"triggered_by"` // manual, auto_crash_detection, panic_button
	ResolvedBy      *uuid.UUID     `json:"resolved_by,omitempty"`
	ResolvedAt      *time.Time     `json:"resolved_at,omitempty"`
	ResolutionNotes string         `json:"resolution_notes,omitempty"`
	NotificationsSent JSONMap      `json:"notifications_sent,omitempty" gorm:"type:jsonb"`
	EmergencyContactsNotified bool  `json:"emergency_contacts_notified" gorm:"default:false"`
	PoliceNotified    bool         `json:"police_notified" gorm:"default:false"`
	AmbulanceCalled   bool         `json:"ambulance_called" gorm:"default:false"`
	AudioRecordingURL string       `json:"audio_recording_url,omitempty"`
	CreatedAt         time.Time    `json:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `json:"-" gorm:"index"`
}

// Trip sharing for live location sharing with trusted contacts
type TripShare struct {
	ID              uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;"`
	RideID          uuid.UUID  `json:"ride_id" gorm:"not null;index"`
	RiderID         uuid.UUID  `json:"rider_id" gorm:"not null"`
	ShareToken      string     `json:"share_token" gorm:"uniqueIndex;not null"`
	ExpiresAt       time.Time  `json:"expires_at"`
	SharedWith      []ShareRecipient `json:"shared_with,omitempty" gorm:"foreignKey:TripShareID"`
	IsActive        bool       `json:"is_active" gorm:"default:true"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type ShareRecipient struct {
	ID           uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;"`
	TripShareID  uuid.UUID  `json:"trip_share_id" gorm:"not null;index"`
	Name         string     `json:"name"`
	Phone        string     `json:"phone"`
	Email        string     `json:"email"`
	NotifiedAt   *time.Time `json:"notified_at,omitempty"`
	ViewedAt     *time.Time `json:"viewed_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// SafetyCheckIn for periodic check-ins during ride
type SafetyCheckIn struct {
	ID         uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;"`
	RideID     uuid.UUID  `json:"ride_id" gorm:"not null;index"`
	UserID     uuid.UUID  `json:"user_id" gorm:"not null"`
	ScheduledAt time.Time `json:"scheduled_at"`
	CheckedInAt *time.Time `json:"checked_in_at,omitempty"`
	IsOverdue  bool       `json:"is_overdue" gorm:"default:false"`
	AutoTriggeredSOS bool  `json:"auto_triggered_sos" gorm:"default:false"`
	CreatedAt  time.Time  `json:"created_at"`
}

// SafetySettings per user
type SafetySettings struct {
	ID                      uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;"`
	UserID                  uuid.UUID  `json:"user_id" gorm:"uniqueIndex;not null"`
	AutoShareTrip           bool       `json:"auto_share_trip" gorm:"default:false"`
	AutoShareContacts       []string   `json:"auto_share_contacts,omitempty" gorm:"type:text[]"`
	CheckInEnabled          bool       `json:"check_in_enabled" gorm:"default:false"`
	CheckInInterval         int        `json:"check_in_interval" gorm:"default:15"` // minutes
	PanicButtonSound        bool       `json:"panic_button_sound" gorm:"default:true"`
	CrashDetectionEnabled   bool       `json:"crash_detection_enabled" gorm:"default:true"`
	FakeCallEnabled         bool       `json:"fake_call_enabled" gorm:"default:false"`
	ShareExactLocation      bool       `json:"share_exact_location" gorm:"default:true"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
}

// IncidentReport for reporting issues after ride
type IncidentReport struct {
	ID          uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;"`
	RideID      uuid.UUID      `json:"ride_id" gorm:"not null;index"`
	ReporterID  uuid.UUID      `json:"reporter_id" gorm:"not null"`
	ReporterType string        `json:"reporter_type"` // rider, driver
	IncidentType string        `json:"incident_type" gorm:"not null"` // harassment, rash_driving, overcharging, etc.
	Severity    string         `json:"severity" gorm:"default:medium"` // low, medium, high, critical
	Description string         `json:"description"`
	EvidenceURLs []string      `json:"evidence_urls,omitempty" gorm:"type:text[]"`
	Status      string         `json:"status" gorm:"default:open"` // open, investigating, resolved, closed
	AssignedTo  *uuid.UUID     `json:"assigned_to,omitempty"`
	Resolution  string         `json:"resolution,omitempty"`
	ResolvedAt  *time.Time     `json:"resolved_at,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

func (s *SOSAlert) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

func (ts *TripShare) BeforeCreate(tx *gorm.DB) error {
	if ts.ID == uuid.Nil {
		ts.ID = uuid.New()
	}
	if ts.ShareToken == "" {
		ts.ShareToken = uuid.New().String()[:8]
	}
	return nil
}

func (sr *ShareRecipient) BeforeCreate(tx *gorm.DB) error {
	if sr.ID == uuid.Nil {
		sr.ID = uuid.New()
	}
	return nil
}

func (sci *SafetyCheckIn) BeforeCreate(tx *gorm.DB) error {
	if sci.ID == uuid.Nil {
		sci.ID = uuid.New()
	}
	return nil
}

func (ss *SafetySettings) BeforeCreate(tx *gorm.DB) error {
	if ss.ID == uuid.Nil {
		ss.ID = uuid.New()
	}
	return nil
}

func (ir *IncidentReport) BeforeCreate(tx *gorm.DB) error {
	if ir.ID == uuid.Nil {
		ir.ID = uuid.New()
	}
	return nil
}

