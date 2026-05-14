package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Rating struct {
	ID          uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;"`
	RideID      uuid.UUID      `json:"ride_id" gorm:"not null;uniqueIndex"`
	RiderID     uuid.UUID      `json:"rider_id" gorm:"not null;index"`
	DriverID    uuid.UUID      `json:"driver_id" gorm:"not null;index"`
	RiderRating int            `json:"rider_rating,omitempty"` // 1-5 stars
	DriverRating int           `json:"driver_rating,omitempty" gorm:"not null"` // 1-5 stars
	RiderReview string         `json:"rider_review,omitempty"`
	DriverReview string       `json:"driver_review,omitempty"`
	RiderRatedAt *time.Time    `json:"rider_rated_at,omitempty"`
	DriverRatedAt *time.Time   `json:"driver_rated_at,omitempty"`
	Categories  RatingCategories `json:"categories,omitempty" gorm:"embedded;embeddedPrefix:cat_"`
	IsReported  bool           `json:"is_reported" gorm:"default:false"`
	ReportReason string       `json:"report_reason,omitempty"`
	ReportDetails string      `json:"report_details,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

type RatingCategories struct {
	Cleanliness  int `json:"cleanliness,omitempty"`  // 1-5
	Punctuality  int `json:"punctuality,omitempty"`  // 1-5
	DrivingSkill int `json:"driving_skill,omitempty"` // 1-5
	Behavior     int `json:"behavior,omitempty"`     // 1-5
	RouteKnowledge int `json:"route_knowledge,omitempty"` // 1-5
}

type DriverRatingSummary struct {
	ID               uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;"`
	DriverID         uuid.UUID  `json:"driver_id" gorm:"uniqueIndex;not null"`
	AverageRating    float64    `json:"average_rating" gorm:"default:5.0"`
	TotalRatings     int        `json:"total_ratings" gorm:"default:0"`
	FiveStarCount    int        `json:"five_star_count" gorm:"default:0"`
	FourStarCount    int        `json:"four_star_count" gorm:"default:0"`
	ThreeStarCount   int        `json:"three_star_count" gorm:"default:0"`
	TwoStarCount     int        `json:"two_star_count" gorm:"default:0"`
	OneStarCount     int        `json:"one_star_count" gorm:"default:0"`
	CleanlinessAvg   float64    `json:"cleanliness_avg" gorm:"default:5.0"`
	PunctualityAvg   float64    `json:"punctuality_avg" gorm:"default:5.0"`
	DrivingSkillAvg  float64    `json:"driving_skill_avg" gorm:"default:5.0"`
	BehaviorAvg      float64    `json:"behavior_avg" gorm:"default:5.0"`
	LastUpdated      time.Time  `json:"last_updated"`
	CreatedAt        time.Time  `json:"created_at"`
}

func (r *Rating) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

func (drs *DriverRatingSummary) BeforeCreate(tx *gorm.DB) error {
	if drs.ID == uuid.Nil {
		drs.ID = uuid.New()
	}
	return nil
}
