package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

type Driver struct {
	ID                 uuid.UUID       `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID             uuid.UUID       `json:"user_id" gorm:"uniqueIndex;not null"`
	User               *User           `json:"user,omitempty" gorm:"foreignKey:UserID"`
	LicenseNumber      string          `json:"license_number" gorm:"uniqueIndex;not null"`
	LicenseImage       string          `json:"license_image"`
	LicenseExpiry      *time.Time      `json:"license_expiry"`
	RCNumber           string          `json:"rc_number" gorm:"uniqueIndex;not null"`
	RCImage            string          `json:"rc_image"`
	AadhaarNumber      string          `json:"-" gorm:"uniqueIndex"` // masked in JSON
	AadhaarImage       string          `json:"aadhaar_image"`
	IsVerified         bool            `json:"is_verified" gorm:"default:false"`
	IsOnline           bool            `json:"is_online" gorm:"default:false"`
	IsActive           bool            `json:"is_active" gorm:"default:true"`
	Rating             float64         `json:"rating" gorm:"default:5.0"`
	TotalRides         int             `json:"total_rides" gorm:"default:0"`
	AcceptanceScore    float64         `json:"acceptance_score" gorm:"default:100.0"`
	CancellationRate   float64         `json:"cancellation_rate" gorm:"default:0.0"`
	CurrentLocation    *DriverLocation `json:"current_location,omitempty" gorm:"-"`
	PreferredLocations pq.StringArray  `json:"preferred_locations,omitempty" gorm:"type:text[]"`
	Languages          pq.StringArray  `json:"languages,omitempty" gorm:"type:text[]"`
	IsFemale           bool            `json:"is_female" gorm:"default:false"`
	VerifiedBy         *uuid.UUID      `json:"verified_by,omitempty"`
	VerifiedAt         *time.Time      `json:"verified_at,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
	DeletedAt          gorm.DeletedAt  `json:"-" gorm:"index"`
}

type DriverLocation struct {
	ID           uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	DriverID     uuid.UUID `json:"driver_id" gorm:"uniqueIndex;not null"`
	Latitude     float64   `json:"lat" gorm:"not null"`
	Longitude    float64   `json:"lng" gorm:"not null"`
	Accuracy     float64   `json:"accuracy"`
	Heading      float64   `json:"heading"`
	Speed        float64   `json:"speed"`
	BatteryLevel int       `json:"battery_level"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Vehicle struct {
	ID           uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	DriverID     uuid.UUID `json:"driver_id" gorm:"not null;index"`
	Type         string    `json:"type" gorm:"not null"` // bike, auto, car
	Make         string    `json:"make"`
	Model        string    `json:"model"`
	Year         int       `json:"year"`
	Color        string    `json:"color"`
	NumberPlate  string    `json:"number_plate" gorm:"uniqueIndex;not null"`
	FuelType     string    `json:"fuel_type"`
	IsActive     bool      `json:"is_active" gorm:"default:true"`
	VehicleImage string    `json:"vehicle_image"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type DriverDocument struct {
	ID              uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	DriverID        uuid.UUID  `json:"driver_id" gorm:"not null;index"`
	Type            string     `json:"type" gorm:"not null"` // license, rc, aadhaar, insurance, permit
	Number          string     `json:"number"`
	ImageURL        string     `json:"image_url"`
	Status          string     `json:"status" gorm:"default:'pending'"` // pending, approved, rejected
	RejectionReason string     `json:"rejection_reason,omitempty"`
	VerifiedBy      *uuid.UUID `json:"verified_by,omitempty"`
	VerifiedAt      *time.Time `json:"verified_at,omitempty"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type DriverEarnings struct {
	ID              uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	DriverID        uuid.UUID `json:"driver_id" gorm:"not null;index"`
	TotalEarnings   float64   `json:"total_earnings" gorm:"default:0"`
	TotalRides      int       `json:"total_rides" gorm:"default:0"`
	PendingAmount   float64   `json:"pending_amount" gorm:"default:0"`
	WithdrawnAmount float64   `json:"withdrawn_amount" gorm:"default:0"`
	CurrentBalance  float64   `json:"current_balance" gorm:"default:0"`
	DailyEarnings   float64   `json:"daily_earnings" gorm:"default:0"`
	WeeklyEarnings  float64   `json:"weekly_earnings" gorm:"default:0"`
	MonthlyEarnings float64   `json:"monthly_earnings" gorm:"default:0"`
	LastUpdated     time.Time `json:"last_updated"`
	CreatedAt       time.Time `json:"created_at"`
}

type DriverStatusLog struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	DriverID  uuid.UUID `json:"driver_id" gorm:"not null;index"`
	IsOnline  bool      `json:"is_online"`
	Latitude  float64   `json:"lat"`
	Longitude float64   `json:"lng"`
	Reason    string    `json:"reason,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func (d *Driver) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	return nil
}

func (dl *DriverLocation) BeforeCreate(tx *gorm.DB) error {
	if dl.ID == uuid.Nil {
		dl.ID = uuid.New()
	}
	return nil
}

func (v *Vehicle) BeforeCreate(tx *gorm.DB) error {
	if v.ID == uuid.Nil {
		v.ID = uuid.New()
	}
	return nil
}

func (dd *DriverDocument) BeforeCreate(tx *gorm.DB) error {
	if dd.ID == uuid.Nil {
		dd.ID = uuid.New()
	}
	return nil
}

func (de *DriverEarnings) BeforeCreate(tx *gorm.DB) error {
	if de.ID == uuid.Nil {
		de.ID = uuid.New()
	}
	return nil
}

func (dsl *DriverStatusLog) BeforeCreate(tx *gorm.DB) error {
	if dsl.ID == uuid.Nil {
		dsl.ID = uuid.New()
	}
	return nil
}
