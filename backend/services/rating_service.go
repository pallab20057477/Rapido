package services

import (
	"fmt"
	"math"
	"time"

	"rapido-backend/database"
	"rapido-backend/models"
	"rapido-backend/utils"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// RatingService handles rating operations
type RatingService struct {
	db *gorm.DB
}

// NewRatingService creates service
func NewRatingService() *RatingService {
	return &RatingService{
		db: database.DB,
	}
}

// SubmitRating submits a rating for a ride
func (s *RatingService) SubmitRating(
	rideID, userID uuid.UUID,
	userType string, // "rider" or "driver"
	rating int,
	review string,
	categories models.RatingCategories,
) (*models.Rating, error) {
	// Validate rating
	if rating < 1 || rating > 5 {
		return nil, fmt.Errorf("rating must be between 1 and 5")
	}

	// Get ride
	var ride models.Ride
	if err := s.db.Where("id = ?", rideID).First(&ride).Error; err != nil {
		return nil, fmt.Errorf("ride not found")
	}

	// Check if ride is completed
	if ride.Status != models.RideStatusCompleted {
		return nil, fmt.Errorf("can only rate completed rides")
	}

	// Check rating window (24 hours after completion)
	if ride.CompletedAt != nil && time.Since(*ride.CompletedAt) > 24*time.Hour {
		return nil, fmt.Errorf("rating window expired (24 hours)")
	}

	// Find or create rating record
	var ratingRecord models.Rating
	result := s.db.Where("ride_id = ?", rideID).First(&ratingRecord)

	now := time.Now()

	if result.Error == gorm.ErrRecordNotFound {
		// Create new rating record
		ratingRecord = models.Rating{
			RideID:   rideID,
			RiderID:  ride.RiderID,
			DriverID: *ride.DriverID,
		}
		if err := s.db.Create(&ratingRecord).Error; err != nil {
			return nil, err
		}
	}

	// Update based on user type
	if userType == "rider" {
		if ride.RiderID != userID {
			return nil, fmt.Errorf("unauthorized: not your ride")
		}
		if ratingRecord.RiderRatedAt != nil {
			return nil, fmt.Errorf("already rated this ride")
		}
		ratingRecord.DriverRating = rating
		ratingRecord.DriverReview = review
		ratingRecord.Categories = categories
		ratingRecord.RiderRatedAt = &now
	} else if userType == "driver" {
		if *ride.DriverID != userID {
			return nil, fmt.Errorf("unauthorized: not your ride")
		}
		if ratingRecord.DriverRatedAt != nil {
			return nil, fmt.Errorf("already rated this ride")
		}
		ratingRecord.RiderRating = rating
		ratingRecord.RiderReview = review
		ratingRecord.DriverRatedAt = &now
	} else {
		return nil, fmt.Errorf("invalid user type")
	}

	// Save rating
	if err := s.db.Save(&ratingRecord).Error; err != nil {
		return nil, err
	}

	// Update driver rating summary
	if userType == "rider" {
		s.updateDriverRatingSummary(ratingRecord.DriverID)
	}

	utils.Info("Rating submitted",
		zap.String("ride_id", rideID.String()),
		zap.String("user_type", userType),
		zap.Int("rating", rating))

	return &ratingRecord, nil
}

// GetDriverReviews gets all reviews for a driver
func (s *RatingService) GetDriverReviews(driverID uuid.UUID, page, limit int) ([]models.Rating, int64, error) {
	var ratings []models.Rating
	var total int64

	offset := (page - 1) * limit

	// Count total
	s.db.Model(&models.Rating{}).Where("driver_id = ? AND driver_rating > 0", driverID).Count(&total)

	// Get ratings
	result := s.db.Where("driver_id = ? AND driver_rating > 0", driverID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&ratings)

	return ratings, total, result.Error
}

// GetDriverRatingSummary gets rating summary for a driver
func (s *RatingService) GetDriverRatingSummary(driverID uuid.UUID) (*models.DriverRatingSummary, error) {
	var summary models.DriverRatingSummary
	result := s.db.Where("driver_id = ?", driverID).First(&summary)

	if result.Error == gorm.ErrRecordNotFound {
		// Create default summary
		summary = models.DriverRatingSummary{
			DriverID:      driverID,
			AverageRating: 5.0,
		}
		if err := s.db.Create(&summary).Error; err != nil {
			return nil, err
		}
	} else if result.Error != nil {
		return nil, result.Error
	}

	return &summary, nil
}

// updateDriverRatingSummary recalculates driver ratings
func (s *RatingService) updateDriverRatingSummary(driverID uuid.UUID) {
	var ratings []models.Rating
	s.db.Where("driver_id = ? AND driver_rating > 0", driverID).Find(&ratings)

	if len(ratings) == 0 {
		return
	}

	// Calculate statistics
	var totalRating, cleanliness, punctuality, driving, behavior float64
	starCounts := make(map[int]int)

	for _, r := range ratings {
		totalRating += float64(r.DriverRating)
		starCounts[r.DriverRating]++

		if r.Categories.Cleanliness > 0 {
			cleanliness += float64(r.Categories.Cleanliness)
		}
		if r.Categories.Punctuality > 0 {
			punctuality += float64(r.Categories.Punctuality)
		}
		if r.Categories.DrivingSkill > 0 {
			driving += float64(r.Categories.DrivingSkill)
		}
		if r.Categories.Behavior > 0 {
			behavior += float64(r.Categories.Behavior)
		}
	}

	count := float64(len(ratings))
	summary := models.DriverRatingSummary{
		DriverID:        driverID,
		AverageRating:   math.Round(totalRating/count*10) / 10,
		TotalRatings:    len(ratings),
		FiveStarCount:   starCounts[5],
		FourStarCount:   starCounts[4],
		ThreeStarCount:  starCounts[3],
		TwoStarCount:    starCounts[2],
		OneStarCount:    starCounts[1],
		CleanlinessAvg:  math.Round(cleanliness/count*10) / 10,
		PunctualityAvg:  math.Round(punctuality/count*10) / 10,
		DrivingSkillAvg: math.Round(driving/count*10) / 10,
		BehaviorAvg:     math.Round(behavior/count*10) / 10,
		LastUpdated:     time.Now(),
	}

	// Upsert
	s.db.Where("driver_id = ?", driverID).Assign(summary).FirstOrCreate(&summary)

	utils.Info("Driver rating summary updated",
		zap.String("driver_id", driverID.String()),
		zap.Float64("average", summary.AverageRating))
}

// ReportRating reports a rating for review
func (s *RatingService) ReportRating(ratingID uuid.UUID, reason, details string) error {
	return s.db.Model(&models.Rating{}).Where("id = ?", ratingID).Updates(map[string]interface{}{
		"is_reported":    true,
		"report_reason":  reason,
		"report_details": details,
	}).Error
}

// GetRatingStatsForDriver gets rating distribution
func (s *RatingService) GetRatingStatsForDriver(driverID uuid.UUID) map[string]interface{} {
	summary, _ := s.GetDriverRatingSummary(driverID)

	return map[string]interface{}{
		"average_rating": summary.AverageRating,
		"total_ratings":  summary.TotalRatings,
		"distribution": map[string]int{
			"5_star": summary.FiveStarCount,
			"4_star": summary.FourStarCount,
			"3_star": summary.ThreeStarCount,
			"2_star": summary.TwoStarCount,
			"1_star": summary.OneStarCount,
		},
		"category_averages": map[string]float64{
			"cleanliness":   summary.CleanlinessAvg,
			"punctuality":   summary.PunctualityAvg,
			"driving_skill": summary.DrivingSkillAvg,
			"behavior":      summary.BehaviorAvg,
		},
	}
}

// GetRideRating gets the current user's rating for a specific ride
func (s *RatingService) GetRideRating(rideID, userID uuid.UUID, userType string) (*models.Rating, error) {
	var rating models.Rating
	if err := s.db.Where("ride_id = ?", rideID).First(&rating).Error; err != nil {
		return nil, fmt.Errorf("rating not found")
	}

	// Verify user has access to this rating
	if userType == "rider" {
		if rating.RiderID != userID {
			return nil, fmt.Errorf("unauthorized: not your ride")
		}
		if rating.RiderRatedAt == nil {
			return nil, fmt.Errorf("you have not rated this ride yet")
		}
	} else if userType == "driver" {
		if rating.DriverID != userID {
			return nil, fmt.Errorf("unauthorized: not your ride")
		}
		if rating.DriverRatedAt == nil {
			return nil, fmt.Errorf("you have not rated this ride yet")
		}
	}

	return &rating, nil
}

// GetPendingReports gets all reported ratings pending admin review
func (s *RatingService) GetPendingReports(page, limit int) ([]models.Rating, int64, error) {
	var ratings []models.Rating
	var total int64

	offset := (page - 1) * limit

	// Count total reported ratings
	if err := s.db.Model(&models.Rating{}).Where("is_reported = ?", true).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get reported ratings
	result := s.db.Where("is_reported = ?", true).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&ratings)

	return ratings, total, result.Error
}

// ResolveReport resolves a reported rating (admin action)
func (s *RatingService) ResolveReport(ratingID uuid.UUID, action, notes string) error {
	var rating models.Rating
	if err := s.db.Where("id = ? AND is_reported = ?", ratingID, true).First(&rating).Error; err != nil {
		return fmt.Errorf("reported rating not found")
	}

	now := time.Now()
	updates := map[string]interface{}{
		"is_reported":     false,
		"report_resolved": true,
		"report_notes":    notes,
		"resolved_at":     &now,
	}

	// If action is "remove", also soft-delete the rating
	if action == "remove" {
		if err := s.db.Delete(&rating).Error; err != nil {
			return fmt.Errorf("failed to remove rating: %w", err)
		}
		// Recalculate driver rating after removal
		s.updateDriverRatingSummary(rating.DriverID)
	} else {
		// Just update the report status
		if err := s.db.Model(&rating).Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to resolve report: %w", err)
		}
	}

	return nil
}

// Global instance
var RatingSvc *RatingService

// InitRatingService initializes service
func InitRatingService() {
	RatingSvc = NewRatingService()
}
