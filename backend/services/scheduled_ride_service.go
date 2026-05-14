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

// ScheduledRideService handles scheduled ride operations
type ScheduledRideService struct {
	db *gorm.DB
}

// NewScheduledRideService creates service
func NewScheduledRideService() *ScheduledRideService {
	return &ScheduledRideService{
		db: database.DB,
	}
}

// ScheduleRide creates a new scheduled ride
func (s *ScheduledRideService) ScheduleRide(
	riderID uuid.UUID,
	pickupLat, pickupLng float64,
	pickupAddress string,
	dropoffLat, dropoffLng float64,
	dropoffAddress string,
	vehicleType string,
	scheduledAt time.Time,
	notes string,
) (*models.ScheduledRide, error) {
	// Validate scheduled time (must be at least 30 minutes in future)
	if time.Until(scheduledAt) < 30*time.Minute {
		return nil, fmt.Errorf("ride must be scheduled at least 30 minutes in advance")
	}

	// Validate scheduled time (max 7 days in advance)
	if scheduledAt.After(time.Now().AddDate(0, 0, 7)) {
		return nil, fmt.Errorf("ride can only be scheduled up to 7 days in advance")
	}

	scheduledRide := &models.ScheduledRide{
		RiderID:        riderID,
		PickupLat:      pickupLat,
		PickupLng:      pickupLng,
		PickupAddress:  pickupAddress,
		DropoffLat:     dropoffLat,
		DropoffLng:     dropoffLng,
		DropoffAddress: dropoffAddress,
		VehicleType:    vehicleType,
		ScheduledAt:    scheduledAt,
		Status:         "pending",
		Notes:          notes,
	}

	if err := s.db.Create(scheduledRide).Error; err != nil {
		return nil, err
	}

	utils.Info("Scheduled ride created",
		zap.String("ride_id", scheduledRide.ID.String()),
		zap.Time("scheduled_at", scheduledAt))

	return scheduledRide, nil
}

// GetScheduledRides gets upcoming scheduled rides for rider
func (s *ScheduledRideService) GetScheduledRides(riderID uuid.UUID) ([]models.ScheduledRide, error) {
	var rides []models.ScheduledRide

	result := s.db.Where("rider_id = ? AND scheduled_at > ? AND status != ?",
		riderID, time.Now(), "cancelled").
		Order("scheduled_at ASC").
		Find(&rides)

	return rides, result.Error
}

// GetScheduledRideByID gets a scheduled ride by ID
func (s *ScheduledRideService) GetScheduledRideByID(rideID, riderID uuid.UUID) (*models.ScheduledRide, error) {
	var ride models.ScheduledRide
	if err := s.db.Where("id = ? AND rider_id = ?", rideID, riderID).First(&ride).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("scheduled ride not found")
		}
		return nil, fmt.Errorf("failed to get scheduled ride: %w", err)
	}
	return &ride, nil
}

// UpdateScheduledRide updates a scheduled ride
func (s *ScheduledRideService) UpdateScheduledRide(rideID, riderID uuid.UUID, req interface{}) (*models.ScheduledRide, error) {
	// Get existing ride
	ride, err := s.GetScheduledRideByID(rideID, riderID)
	if err != nil {
		return nil, err
	}

	// Can only update if status is pending
	if ride.Status != "pending" {
		return nil, fmt.Errorf("can only update pending scheduled rides")
	}

	// Validate new time (at least 30 min in future)
	if ride.ScheduledAt.Before(time.Now().Add(30 * time.Minute)) {
		return nil, fmt.Errorf("ride must be scheduled at least 30 minutes in advance")
	}

	// Update ride
	if err := s.db.Save(ride).Error; err != nil {
		return nil, fmt.Errorf("failed to update ride: %w", err)
	}

	return ride, nil
}

// CancelScheduledRide cancels a scheduled ride
func (s *ScheduledRideService) CancelScheduledRide(rideID, riderID uuid.UUID) error {
	var ride models.ScheduledRide

	if err := s.db.Where("id = ? AND rider_id = ?", rideID, riderID).First(&ride).Error; err != nil {
		return fmt.Errorf("scheduled ride not found")
	}

	if !ride.CanCancel() {
		return fmt.Errorf("cannot cancel within 2 hours of scheduled time")
	}

	if ride.Status == "assigned" && ride.RideID != nil {
		// Cancel the actual ride too
		s.db.Model(&models.Ride{}).Where("id = ?", *ride.RideID).
			Update("status", models.RideStatusCancelled)
	}

	return s.db.Model(&ride).Update("status", "cancelled").Error
}

// ProcessScheduledRides processes rides that need notification (runs every 5 minutes)
func (s *ScheduledRideService) ProcessScheduledRides() {
	// Find rides scheduled in next 15-20 minutes that haven't been notified
	var rides []models.ScheduledRide

	now := time.Now()
	notifyWindowStart := now.Add(15 * time.Minute)
	notifyWindowEnd := now.Add(20 * time.Minute)

	s.db.Where("status = ? AND scheduled_at BETWEEN ? AND ? AND notification_sent_at IS NULL",
		"pending", notifyWindowStart, notifyWindowEnd).
		Find(&rides)

	for _, ride := range rides {
		go s.notifyAndMatchRide(&ride)
	}
}

// notifyAndMatchRide sends notification and starts matching
func (s *ScheduledRideService) notifyAndMatchRide(ride *models.ScheduledRide) {
	// Send notification to rider
	// TODO: Send push notification

	// Update status
	now := time.Now()
	s.db.Model(ride).Updates(map[string]interface{}{
		"status":               "notified",
		"notification_sent_at": now,
	})

	// Start matching process
	matchingService := NewMatchingService()

	// Create a temporary ride request for matching
	tempRide := &models.Ride{
		RiderID:     ride.RiderID,
		VehicleType: ride.VehicleType,
		Status:      models.RideStatusRequested,
		Pickup: models.Location{
			Latitude:  ride.PickupLat,
			Longitude: ride.PickupLng,
			Address:   ride.PickupAddress,
		},
		Dropoff: models.Location{
			Latitude:  ride.DropoffLat,
			Longitude: ride.DropoffLng,
			Address:   ride.DropoffAddress,
		},
	}

	// Start async matching
	go matchingService.StartMatchingProcess(tempRide)

	utils.Info("Processed scheduled ride notification",
		zap.String("ride_id", ride.ID.String()))
}

// StartScheduledRideProcessor starts background processor
func (s *ScheduledRideService) StartScheduledRideProcessor() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			s.ProcessScheduledRides()
		}
	}()

	utils.Info("Scheduled ride processor started")
}

// Global instance
var ScheduledRideSvc *ScheduledRideService

// InitScheduledRideService initializes service
func InitScheduledRideService() {
	ScheduledRideSvc = NewScheduledRideService()
	ScheduledRideSvc.StartScheduledRideProcessor()
}
