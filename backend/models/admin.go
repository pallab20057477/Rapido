package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Admin role constants
const (
	AdminRoleSuperAdmin = "super_admin"
	AdminRoleAdmin      = "admin"
	AdminRoleSupport    = "support"
	AdminRoleFinance    = "finance"
	AdminRoleOperations = "operations"
)

// Admin status
const (
	AdminStatusActive    = "active"
	AdminStatusInactive  = "inactive"
	AdminStatusSuspended = "suspended"
)

type Admin struct {
	ID          uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;"`
	UserID      uuid.UUID      `json:"user_id" gorm:"uniqueIndex;not null"`
	User        *User          `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Role        string         `json:"role" gorm:"default:admin"`
	Status      string         `json:"status" gorm:"default:active"`
	Department  string         `json:"department,omitempty"`
	Permissions []string       `json:"permissions,omitempty" gorm:"type:text[]"`
	LastLoginAt *time.Time     `json:"last_login_at,omitempty"`
	LoginIP     string         `json:"login_ip,omitempty"`
	CreatedBy   *uuid.UUID     `json:"created_by,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

// AdminActivityLog for audit trail
type AdminActivityLog struct {
	ID          uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;"`
	AdminID     uuid.UUID  `json:"admin_id" gorm:"not null;index"`
	Action      string     `json:"action" gorm:"not null"`
	EntityType  string     `json:"entity_type,omitempty"` // driver, ride, user, etc.
	EntityID    *uuid.UUID `json:"entity_id,omitempty"`
	OldValues   JSONMap    `json:"old_values,omitempty" gorm:"type:jsonb"`
	NewValues   JSONMap    `json:"new_values,omitempty" gorm:"type:jsonb"`
	Description string     `json:"description,omitempty"`
	IP          string     `json:"ip,omitempty"`
	UserAgent   string     `json:"user_agent,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// SystemSettings for app-wide configuration
type SystemSettings struct {
	ID             uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;"`
	Key            string     `json:"key" gorm:"uniqueIndex;not null"`
	Value          string     `json:"value"`
	DataType       string     `json:"data_type" gorm:"default:string"` // string, number, boolean, json
	Category       string     `json:"category" gorm:"default:general"`
	Description    string     `json:"description,omitempty"`
	IsEditable     bool       `json:"is_editable" gorm:"default:true"`
	LastModifiedBy *uuid.UUID `json:"last_modified_by,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// PromoCode for discounts
type PromoCode struct {
	ID                     uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;"`
	Code                   string         `json:"code" gorm:"uniqueIndex;not null"`
	Description            string         `json:"description"`
	DiscountType           string         `json:"discount_type" gorm:"not null"` // percentage, fixed
	DiscountValue          float64        `json:"discount_value" gorm:"not null"`
	MaxDiscount            float64        `json:"max_discount" gorm:"default:0"`
	MinRideAmount          float64        `json:"min_ride_amount" gorm:"default:0"`
	MaxUses                int            `json:"max_uses" gorm:"default:0"` // 0 = unlimited
	UsesCount              int            `json:"uses_count" gorm:"default:0"`
	MaxUsesPerUser         int            `json:"max_uses_per_user" gorm:"default:1"`
	ApplicableCities       []string       `json:"applicable_cities,omitempty" gorm:"type:text[]"`
	ApplicableVehicleTypes []string       `json:"applicable_vehicle_types,omitempty" gorm:"type:text[]"`
	StartDate              *time.Time     `json:"start_date,omitempty"`
	EndDate                *time.Time     `json:"end_date,omitempty"`
	IsActive               bool           `json:"is_active" gorm:"default:true"`
	CreatedBy              uuid.UUID      `json:"created_by"`
	CreatedAt              time.Time      `json:"created_at"`
	UpdatedAt              time.Time      `json:"updated_at"`
	DeletedAt              gorm.DeletedAt `json:"-" gorm:"index"`
}

// PromoCodeUsage tracking
type PromoCodeUsage struct {
	ID             uuid.UUID `json:"id" gorm:"type:uuid;primary_key;"`
	PromoCodeID    uuid.UUID `json:"promo_code_id" gorm:"not null;index"`
	UserID         uuid.UUID `json:"user_id" gorm:"not null;index"`
	RideID         uuid.UUID `json:"ride_id" gorm:"not null"`
	DiscountAmount float64   `json:"discount_amount"`
	UsedAt         time.Time `json:"used_at"`
}

// City for operational areas
type City struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key;"`
	Name      string    `json:"name" gorm:"not null"`
	State     string    `json:"state"`
	Country   string    `json:"country" gorm:"default:India"`
	Currency  string    `json:"currency" gorm:"default:INR"`
	IsActive  bool      `json:"is_active" gorm:"default:true"`
	Latitude  float64   `json:"lat"`
	Longitude float64   `json:"lng"`
	RadiusKM  float64   `json:"radius_km"`
	Timezone  string    `json:"timezone" gorm:"default:Asia/Kolkata"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (a *Admin) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

func (aal *AdminActivityLog) BeforeCreate(tx *gorm.DB) error {
	if aal.ID == uuid.Nil {
		aal.ID = uuid.New()
	}
	return nil
}

func (ss *SystemSettings) BeforeCreate(tx *gorm.DB) error {
	if ss.ID == uuid.Nil {
		ss.ID = uuid.New()
	}
	return nil
}

func (pc *PromoCode) BeforeCreate(tx *gorm.DB) error {
	if pc.ID == uuid.Nil {
		pc.ID = uuid.New()
	}
	return nil
}

func (pcu *PromoCodeUsage) BeforeCreate(tx *gorm.DB) error {
	if pcu.ID == uuid.Nil {
		pcu.ID = uuid.New()
	}
	return nil
}

func (c *City) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}
