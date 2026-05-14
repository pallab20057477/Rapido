package services

import (
	"fmt"
	"time"

	"rapido-backend/database"
	"rapido-backend/models"
	"rapido-backend/utils"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// IncentiveService handles driver incentives
type IncentiveService struct {
	db *gorm.DB
}

// NewIncentiveService creates service
func NewIncentiveService() *IncentiveService {
	return &IncentiveService{
		db: database.DB,
	}
}

// GetActiveIncentives gets all active incentives for a driver
func (s *IncentiveService) GetActiveIncentives(driverID uuid.UUID) ([]models.DriverIncentive, error) {
	var driverIncentives []models.DriverIncentive

	result := s.db.Preload("Incentive").
		Joins("JOIN incentives ON incentives.id = driver_incentives.incentive_id").
		Where("driver_incentives.driver_id = ? AND driver_incentives.status IN ? AND incentives.is_active = ?",
			driverID, []string{"in_progress", "completed"}, true).
		Find(&driverIncentives)

	return driverIncentives, result.Error
}

// GetWeeklyTarget gets or creates weekly target for driver
func (s *IncentiveService) GetWeeklyTarget(driverID uuid.UUID) (*models.WeeklyTarget, error) {
	// Calculate week boundaries (Monday to Sunday)
	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	weekStart := now.AddDate(0, 0, -weekday+1).Truncate(24 * time.Hour)
	weekEnd := weekStart.AddDate(0, 0, 7).Add(-time.Second)

	var target models.WeeklyTarget
	result := s.db.Where("driver_id = ? AND week_start = ?", driverID, weekStart).First(&target)

	if result.Error == gorm.ErrRecordNotFound {
		// Get default targets from config
		target = models.WeeklyTarget{
			DriverID:       driverID,
			WeekStart:      weekStart,
			WeekEnd:        weekEnd,
			TargetRides:    20, // Default: 20 rides per week
			TargetHours:    40, // Default: 40 hours per week
			TargetEarnings: 5000.0, // Default: ₹5000
		}
		if err := s.db.Create(&target).Error; err != nil {
			return nil, err
		}
	} else if result.Error != nil {
		return nil, result.Error
	}

	return &target, nil
}

// TrackRideCompletion updates incentive progress after ride completion
func (s *IncentiveService) TrackRideCompletion(driverID uuid.UUID, rideEarnings float64) {
	// Update weekly target
	weekTarget, _ := s.GetWeeklyTarget(driverID)
	if weekTarget != nil {
		weekTarget.CompletedRides++
		weekTarget.ActualEarnings += rideEarnings
		weekTarget.UpdatedAt = time.Now()

		// Check if weekly target met
		if weekTarget.CompletedRides >= weekTarget.TargetRides &&
			weekTarget.ActualEarnings >= weekTarget.TargetEarnings {
			weekTarget.Status = "target_met"
		}

		s.db.Save(weekTarget)
	}

	// Update active incentives
	var driverIncentives []models.DriverIncentive
	s.db.Preload("Incentive").
		Where("driver_id = ? AND status = ?", driverID, "in_progress").
		Find(&driverIncentives)

	now := time.Now()
	for _, di := range driverIncentives {
		// Check if incentive expired
		if now.After(di.Incentive.EndDate) {
			di.Status = "expired"
			s.db.Save(&di)
			continue
		}

		// Update progress
		di.Progress++
		if di.Incentive.BonusPerRide > 0 {
			di.EarnedAmount += di.Incentive.BonusPerRide
		}

		// Check if target met
		if di.Progress >= di.Target {
			di.Status = "completed"
			di.EarnedAmount = di.Incentive.RewardAmount
			completedAt := time.Now()
			di.CompletedAt = &completedAt

			utils.Info("Driver completed incentive",
				zap.String("driver_id", driverID.String()),
				zap.String("incentive_id", di.IncentiveID.String()),
				zap.Float64("earned", di.EarnedAmount))
		}

		s.db.Save(&di)
	}
}

// ClaimIncentive marks an incentive as claimed
func (s *IncentiveService) ClaimIncentive(driverID, incentiveID uuid.UUID) error {
	var di models.DriverIncentive
	result := s.db.Where("driver_id = ? AND incentive_id = ?", driverID, incentiveID).First(&di)
	if result.Error != nil {
		return result.Error
	}

	if di.Status != "completed" {
		return fmt.Errorf("incentive not completed yet")
	}

	now := time.Now()
	return s.db.Model(&di).Updates(map[string]interface{}{
		"status":     "claimed",
		"claimed_at": now,
	}).Error
}

// CreateIncentive creates a new incentive (admin)
func (s *IncentiveService) CreateIncentive(incentive *models.Incentive) error {
	return s.db.Create(incentive).Error
}

// AssignIncentiveToDriver assigns an incentive to eligible drivers
func (s *IncentiveService) AssignIncentiveToDriver(incentiveID, driverID uuid.UUID) error {
	// Check if already assigned
	var existing models.DriverIncentive
	if err := s.db.Where("incentive_id = ? AND driver_id = ?", incentiveID, driverID).First(&existing).Error; err == nil {
		return fmt.Errorf("incentive already assigned")
	}

	// Get incentive details
	var incentive models.Incentive
	if err := s.db.Where("id = ?", incentiveID).First(&incentive).Error; err != nil {
		return err
	}

	di := models.DriverIncentive{
		DriverID:    driverID,
		IncentiveID: incentiveID,
		Target:      incentive.TargetRides,
		Status:      "in_progress",
	}

	return s.db.Create(&di).Error
}

// GetIncentiveStats gets incentive statistics (admin)
func (s *IncentiveService) GetIncentiveStats() map[string]interface{} {
	var stats struct {
		ActiveIncentives   int64
		TotalAssigned      int64
		Completed          int64
		Claimed            int64
		TotalAmountEarned  float64
		TotalAmountClaimed float64
	}

	s.db.Model(&models.Incentive{}).Where("is_active = ?", true).Count(&stats.ActiveIncentives)
	s.db.Model(&models.DriverIncentive{}).Count(&stats.TotalAssigned)
	s.db.Model(&models.DriverIncentive{}).Where("status = ?", "completed").Count(&stats.Completed)
	s.db.Model(&models.DriverIncentive{}).Where("status = ?", "claimed").Count(&stats.Claimed)

	// Sum earnings
	var earnedResult struct{ Total float64 }
	s.db.Raw("SELECT COALESCE(SUM(earned_amount), 0) as total FROM driver_incentives WHERE status IN ?", []string{"completed", "claimed"}).Scan(&earnedResult)
	stats.TotalAmountEarned = earnedResult.Total

	var claimedResult struct{ Total float64 }
	s.db.Raw("SELECT COALESCE(SUM(earned_amount), 0) as total FROM driver_incentives WHERE status = ?", "claimed").Scan(&claimedResult)
	stats.TotalAmountClaimed = claimedResult.Total

	return map[string]interface{}{
		"active_incentives":    stats.ActiveIncentives,
		"total_assigned":     stats.TotalAssigned,
		"completed":          stats.Completed,
		"claimed":            stats.Claimed,
		"total_amount_earned":  stats.TotalAmountEarned,
		"total_amount_claimed": stats.TotalAmountClaimed,
	}
}

// Global instance
var IncentiveSvc *IncentiveService

// InitIncentiveService initializes service
func InitIncentiveService() {
	IncentiveSvc = NewIncentiveService()
}
