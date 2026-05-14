package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Incentive represents a driver incentive program
type Incentive struct {
	ID                uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;"`
	Title             string         `json:"title" gorm:"not null"`
	Description       string         `json:"description" gorm:"not null"`
	Type              string         `json:"type" gorm:"not null"` // weekly_target, streak, peak_hour, referral
	StartDate         time.Time      `json:"start_date" gorm:"not null"`
	EndDate           time.Time      `json:"end_date" gorm:"not null"`
	TargetRides       int            `json:"target_rides,omitempty"`    // For ride-based incentives
	TargetHours       int            `json:"target_hours,omitempty"`    // For hour-based incentives
	TargetEarnings    float64        `json:"target_earnings,omitempty"` // For earning-based
	RewardAmount      float64        `json:"reward_amount" gorm:"not null"`
	BonusPerRide      float64        `json:"bonus_per_ride,omitempty"` // Extra per ride
	ValidVehicleTypes []string       `json:"valid_vehicle_types,omitempty" gorm:"type:text[]"`
	ValidCities       []string       `json:"valid_cities,omitempty" gorm:"type:text[]"`
	IsActive          bool           `json:"is_active" gorm:"default:true"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `json:"-" gorm:"index"`
}

// DriverIncentive tracks driver progress on incentives
type DriverIncentive struct {
	ID           uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;"`
	DriverID     uuid.UUID  `json:"driver_id" gorm:"not null;index"`
	IncentiveID  uuid.UUID  `json:"incentive_id" gorm:"not null;index"`
	Incentive    *Incentive `json:"incentive,omitempty" gorm:"foreignKey:IncentiveID"`
	Progress     int        `json:"progress" gorm:"default:0"`
	Target       int        `json:"target" gorm:"not null"`
	Status       string     `json:"status" gorm:"default:in_progress"` // in_progress, completed, claimed, expired
	EarnedAmount float64    `json:"earned_amount" gorm:"default:0"`
	ClaimedAt    *time.Time `json:"claimed_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// WeeklyTarget tracks driver's weekly performance
type WeeklyTarget struct {
	ID              uuid.UUID `json:"id" gorm:"type:uuid;primary_key;"`
	DriverID        uuid.UUID `json:"driver_id" gorm:"not null;index"`
	WeekStart       time.Time `json:"week_start" gorm:"not null;index"`
	WeekEnd         time.Time `json:"week_end" gorm:"not null"`
	TargetRides     int       `json:"target_rides" gorm:"default:0"`
	CompletedRides  int       `json:"completed_rides" gorm:"default:0"`
	TargetHours     int       `json:"target_hours" gorm:"default:0"`
	CompletedHours  float64   `json:"completed_hours" gorm:"default:0"`
	TargetEarnings  float64   `json:"target_earnings" gorm:"default:0"`
	ActualEarnings  float64   `json:"actual_earnings" gorm:"default:0"`
	IncentiveEarned float64   `json:"incentive_earned" gorm:"default:0"`
	Status          string    `json:"status" gorm:"default:in_progress"` // in_progress, target_met, completed
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// TableName for Incentive
func (Incentive) TableName() string {
	return "incentives"
}

// TableName for DriverIncentive
func (DriverIncentive) TableName() string {
	return "driver_incentives"
}

// TableName for WeeklyTarget
func (WeeklyTarget) TableName() string {
	return "weekly_targets"
}

// IsActive checks if incentive is currently active
func (i *Incentive) IsActiveNow() bool {
	now := time.Now()
	return i.IsActive && now.After(i.StartDate) && now.Before(i.EndDate)
}

// GetProgressPercentage calculates progress
func (di *DriverIncentive) GetProgressPercentage() float64 {
	if di.Target == 0 {
		return 0
	}
	return float64(di.Progress) / float64(di.Target) * 100
}
