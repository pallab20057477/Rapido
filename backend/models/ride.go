package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Ride status constants
const (
	RideStatusRequested      = "requested"
	RideStatusDriverAssigned = "driver_assigned"
	RideStatusDriverArrived  = "driver_arrived"
	RideStatusOngoing        = "ongoing"
	RideStatusCompleted      = "completed"
	RideStatusCancelled      = "cancelled"
	RideStatusNoDriverFound  = "no_driver_found"
)

// Cancellation reasons
const (
	CancelReasonRiderCancelled    = "rider_cancelled"
	CancelReasonDriverCancelled   = "driver_cancelled"
	CancelReasonNoDriverAvailable = "no_driver_available"
	CancelReasonDriverNotFound    = "driver_not_found"
	CancelReasonOther             = "other"
)

// Vehicle types
const (
	VehicleTypeBike  = "bike"
	VehicleTypeAuto  = "auto"
	VehicleTypeCarGo = "car_go"
	VehicleTypeCarX  = "car_x"
)

type Ride struct {
	ID                 uuid.UUID       `json:"id" gorm:"type:uuid;primary_key;"`
	RiderID            uuid.UUID       `json:"rider_id" gorm:"not null;index"`
	Rider              *User           `json:"rider,omitempty" gorm:"foreignKey:RiderID;references:ID"`
	DriverID           *uuid.UUID      `json:"driver_id,omitempty" gorm:"index"`
	Driver             *Driver         `json:"driver,omitempty" gorm:"foreignKey:DriverID"`
	VehicleID          *uuid.UUID      `json:"vehicle_id,omitempty"`
	Vehicle            *Vehicle        `json:"vehicle,omitempty" gorm:"foreignKey:VehicleID"`
	Status             string          `json:"status" gorm:"default:requested;index"`
	VehicleType        string          `json:"vehicle_type" gorm:"not null"`
	Pickup             Location        `json:"pickup" gorm:"embedded;embeddedPrefix:pickup_"`
	Dropoff            Location        `json:"dropoff" gorm:"embedded;embeddedPrefix:dropoff_"`
	EstimatedDistance  float64         `json:"estimated_distance"` // km
	EstimatedDuration  int             `json:"estimated_duration"` // minutes
	EstimatedFare      float64         `json:"estimated_fare"`
	ActualDistance     float64         `json:"actual_distance"`
	ActualDuration     int             `json:"actual_duration"`
	FinalFare          float64         `json:"final_fare"`
	BaseFare           float64         `json:"base_fare"`
	PerKmRate          float64         `json:"per_km_rate"`
	PerMinRate         float64         `json:"per_min_rate"`
	SurgeMultiplier    float64         `json:"surge_multiplier" gorm:"default:1.0"`
	SurgeAmount        float64         `json:"surge_amount" gorm:"default:0"`
	PlatformFee        float64         `json:"platform_fee"`
	TaxAmount          float64         `json:"tax_amount"`
	PromoCode          string          `json:"promo_code,omitempty"`
	DiscountAmount     float64         `json:"discount_amount" gorm:"default:0"`
	PaymentMethod      string          `json:"payment_method" gorm:"default:cash"`    // cash, upi, card, wallet
	PaymentStatus      string          `json:"payment_status" gorm:"default:pending"` // pending, completed, failed, refunded
	RideOTP            string          `json:"ride_otp,omitempty"`
	IdempotencyKey     string          `json:"-" gorm:"uniqueIndex"`
	Preferences        RidePreferences `json:"preferences,omitempty" gorm:"embedded;embeddedPrefix:pref_"`
	CancellationReason string          `json:"cancellation_reason,omitempty"`
	CancelledBy        *uuid.UUID      `json:"cancelled_by,omitempty"`
	CancellationTime   *time.Time      `json:"cancellation_time,omitempty"`
	CancellationFee    float64         `json:"cancellation_fee" gorm:"default:0"`
	RequestedAt        time.Time       `json:"requested_at"`
	AcceptedAt         *time.Time      `json:"accepted_at,omitempty"`
	ArrivedAt          *time.Time      `json:"arrived_at,omitempty"`
	StartedAt          *time.Time      `json:"started_at,omitempty"`
	CompletedAt        *time.Time      `json:"completed_at,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
	DeletedAt          gorm.DeletedAt  `json:"-" gorm:"index"`
}

// RidePreferences represents ride booking preferences
type RidePreferences struct {
	ACRequired       bool `json:"ac_required" gorm:"default:false"`
	FemaleDriverOnly bool `json:"female_driver_only" gorm:"default:false"`
	LuggageSpace     bool `json:"luggage_space" gorm:"default:false"`
	SilenceMode      bool `json:"silence_mode" gorm:"default:false"`
	Music            bool `json:"music" gorm:"default:false"`
}

// MatchesDriver checks if driver matches ride preferences
func (rp RidePreferences) MatchesDriver(driver Driver) bool {
	// Check female driver preference
	if rp.FemaleDriverOnly && !driver.IsFemale {
		return false
	}
	// Add other preference checks as needed
	return true
}

type RideLocation struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key;"`
	RideID    uuid.UUID `json:"ride_id" gorm:"not null;index"`
	Latitude  float64   `json:"lat" gorm:"not null"`
	Longitude float64   `json:"lng" gorm:"not null"`
	Accuracy  float64   `json:"accuracy"`
	Speed     float64   `json:"speed"`
	Heading   float64   `json:"heading"`
	Altitude  float64   `json:"altitude"`
	CreatedAt time.Time `json:"created_at"`
}

type RideRequestLog struct {
	ID         uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;"`
	RideID     uuid.UUID  `json:"ride_id" gorm:"not null;index"`
	DriverID   uuid.UUID  `json:"driver_id" gorm:"not null;index"`
	Status     string     `json:"status" gorm:"default:pending"` // pending, accepted, rejected, timeout
	RejectedAt *time.Time `json:"rejected_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type RideMatch struct {
	ID              uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;"`
	RideID          uuid.UUID  `json:"ride_id" gorm:"not null;index"`
	DriverID        uuid.UUID  `json:"driver_id" gorm:"not null;index"`
	Distance        float64    `json:"distance"` // km from pickup
	ETA             int        `json:"eta"`      // minutes to pickup
	DriverRating    float64    `json:"driver_rating"`
	AcceptanceScore float64    `json:"acceptance_score"`
	MatchScore      float64    `json:"match_score"`
	NotifiedAt      time.Time  `json:"notified_at"`
	RespondedAt     *time.Time `json:"responded_at,omitempty"`
	Response        string     `json:"response,omitempty"` // accepted, rejected, timeout
	CreatedAt       time.Time  `json:"created_at"`
}

// SurgePricing represents dynamic pricing multipliers
type SurgePricing struct {
	ID         uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;"`
	AreaName   string     `json:"area_name" gorm:"not null;index"`
	Latitude   float64    `json:"lat" gorm:"not null"`
	Longitude  float64    `json:"lng" gorm:"not null"`
	RadiusKM   float64    `json:"radius_km" gorm:"default:3"`
	Multiplier float64    `json:"multiplier" gorm:"default:1.0"`
	IsActive   bool       `json:"is_active" gorm:"default:false"`
	Reason     string     `json:"reason,omitempty"`
	StartTime  *time.Time `json:"start_time,omitempty"`
	EndTime    *time.Time `json:"end_time,omitempty"`
	CreatedBy  uuid.UUID  `json:"created_by"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// Fares for different vehicle types
type FareConfig struct {
	ID              uuid.UUID `json:"id" gorm:"type:uuid;primary_key;"`
	VehicleType     string    `json:"vehicle_type" gorm:"uniqueIndex;not null"`
	BaseFare        float64   `json:"base_fare"`
	PerKmRate       float64   `json:"per_km_rate"`
	PerMinRate      float64   `json:"per_min_rate"`
	MinFare         float64   `json:"min_fare"`
	MaxFare         float64   `json:"max_fare"`
	PlatformFee     float64   `json:"platform_fee"`
	ServiceFee      float64   `json:"service_fee"`
	NightMultiplier float64   `json:"night_multiplier" gorm:"default:1.0"`
	IsActive        bool      `json:"is_active" gorm:"default:true"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (r *Ride) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	if r.RequestedAt.IsZero() {
		r.RequestedAt = time.Now()
	}
	return nil
}

func (rl *RideLocation) BeforeCreate(tx *gorm.DB) error {
	if rl.ID == uuid.Nil {
		rl.ID = uuid.New()
	}
	return nil
}

func (rrl *RideRequestLog) BeforeCreate(tx *gorm.DB) error {
	if rrl.ID == uuid.Nil {
		rrl.ID = uuid.New()
	}
	return nil
}

func (rm *RideMatch) BeforeCreate(tx *gorm.DB) error {
	if rm.ID == uuid.Nil {
		rm.ID = uuid.New()
	}
	return nil
}

func (sp *SurgePricing) BeforeCreate(tx *gorm.DB) error {
	if sp.ID == uuid.Nil {
		sp.ID = uuid.New()
	}
	return nil
}

func (fc *FareConfig) BeforeCreate(tx *gorm.DB) error {
	if fc.ID == uuid.Nil {
		fc.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name for Ride
func (Ride) TableName() string {
	return "rides"
}
