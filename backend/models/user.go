package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PublicUser is the shared account identity for hotel and ride users.
// The legacy User alias is kept so existing ride code continues to compile.
// PublicUser represents a user in the system
type PublicUser struct {
	ID                uuid.UUID          `json:"id" gorm:"type:uuid;primaryKey;"`
	Name              string             `json:"name"`
	Email             string             `json:"email"`
	Phone             string             `json:"phone"`
	PasswordHash      string             `json:"-"`
	Provider          string             `json:"provider"`
	ProviderID        string             `json:"provider_id,omitempty"`
	EmailVerified     bool               `json:"email_verified"`
	ProfileImage      string             `json:"profile_image"`
	Role              string             `json:"role"`
	GoogleID          string             `json:"-"`
	IsActive          bool               `json:"is_active"`
	Latitude          float64            `json:"latitude,omitempty"`
	Longitude         float64            `json:"longitude,omitempty"`
	Address           string             `json:"address,omitempty"`
	LocationUpdatedAt time.Time          `json:"location_updated_at,omitempty"`
	EmergencyContacts []EmergencyContact `json:"emergency_contacts,omitempty" gorm:"foreignKey:UserID"`
	CreatedAt         time.Time          `json:"created_at"`
	UpdatedAt         time.Time          `json:"updated_at"`
}

// TableName specifies the table name for PublicUser
func (PublicUser) TableName() string {
	return "public_users"
}

type User = PublicUser

type Location struct {
	Latitude  float64   `json:"lat"`
	Longitude float64   `json:"lng"`
	Address   string    `json:"address,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

type OTP struct {
	ID        uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;"`
	Phone     string     `json:"phone" gorm:"not null;index"`
	Code      string     `json:"code" gorm:"not null"`
	Purpose   string     `json:"purpose" gorm:"default:login"` // login, ride, withdrawal
	ExpiresAt time.Time  `json:"expires_at" gorm:"not null"`
	UsedAt    *time.Time `json:"used_at"`
	CreatedAt time.Time  `json:"created_at"`
}

type RefreshToken struct {
	ID        uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;"`
	UserID    uuid.UUID  `json:"user_id" gorm:"not null;index"`
	Token     string     `json:"token" gorm:"uniqueIndex;not null"`
	ExpiresAt time.Time  `json:"expires_at" gorm:"not null"`
	RevokedAt *time.Time `json:"revoked_at"`
	CreatedAt time.Time  `json:"created_at"`
}

func (u *PublicUser) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

func (o *OTP) BeforeCreate(tx *gorm.DB) error {
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	return nil
}

func (r *RefreshToken) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

func (e *EmergencyContact) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	return nil
}
