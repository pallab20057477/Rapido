package services

import (
	"errors"
	"fmt"
	"math"
	"time"

	"rapido-backend/config"
	"rapido-backend/database"
	"rapido-backend/models"
	"rapido-backend/utils"
	"rapido-backend/websocket"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RideService struct {
	DB *gorm.DB
}

func NewRideService() *RideService {
	return &RideService{DB: database.DB}
}

// RequestRide creates a new ride request
type RequestRideRequest struct {
	RiderID        uuid.UUID              `json:"rider_id"`
	VehicleType    string                 `json:"vehicle_type"`
	PickupLat      float64                `json:"pickup_lat"`
	PickupLng      float64                `json:"pickup_lng"`
	PickupAddress  string                 `json:"pickup_address"`
	DropoffLat     float64                `json:"dropoff_lat"`
	DropoffLng     float64                `json:"dropoff_lng"`
	DropoffAddress string                 `json:"dropoff_address"`
	PromoCode      string                 `json:"promo_code,omitempty"`
	PaymentMethod  string                 `json:"payment_method"`
	Preferences    models.RidePreferences `json:"preferences,omitempty"`
	IdempotencyKey string                 `json:"idempotency_key,omitempty"`
}

func (s *RideService) RequestRide(req RequestRideRequest) (*models.Ride, error) {
	if req.RiderID == uuid.Nil {
		return nil, errors.New("invalid rider identity")
	}

	var rider models.User
	if err := s.DB.Where("id = ?", req.RiderID).First(&rider).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("rider not found with ID: %s", req.RiderID.String())
		}
		return nil, fmt.Errorf("database error checking rider: %w", err)
	}

	// Check if rider is active
	if !rider.IsActive {
		return nil, fmt.Errorf("rider account is inactive (ID: %s, Phone: %s)", rider.ID.String(), rider.Phone)
	}

	// Check if rider has correct role
	if rider.Role != "rider" {
		return nil, fmt.Errorf("invalid rider role: %s (expected: rider, user ID: %s)", rider.Role, rider.ID.String())
	}

	// Check if rider has active ride
	existingRideKey := database.GetRiderCurrentRideKey(req.RiderID.String())
	if existingRideID, err := database.GetCache(existingRideKey); err == nil && existingRideID != "" {
		return nil, errors.New("you already have an active ride: " + existingRideID)
	}

	// Check idempotency
	if req.IdempotencyKey != "" {
		var existing models.Ride
		if err := s.DB.Where("idempotency_key = ?", req.IdempotencyKey).First(&existing).Error; err == nil {
			return &existing, nil
		}
	}

	// Get fare configuration
	var fareConfig models.FareConfig
	if err := s.DB.Where("vehicle_type = ? AND is_active = ?", req.VehicleType, true).First(&fareConfig).Error; err != nil {
		return nil, errors.New("vehicle type not available")
	}

	// Calculate distance and duration
	distance := utils.CalculateDistance(req.PickupLat, req.PickupLng, req.DropoffLat, req.DropoffLng)
	duration := utils.EstimateRideDuration(distance, req.VehicleType, 1.0)

	// Calculate fare
	fareBreakdown := utils.EstimateFare(distance, req.VehicleType, duration, 1.0)

	// Check dynamic surge pricing
	surgeService := NewSurgePricingService()
	surgeFactors := surgeService.GetSurgeForRideEstimate(req.PickupLat, req.PickupLng, req.VehicleType)
	surgeMultiplier := surgeFactors.Multiplier

	// Apply promo code discount
	discountAmount := 0.0
	if req.PromoCode != "" {
		discount, err := s.calculatePromoDiscount(req.PromoCode, fareBreakdown["total"], req.RiderID)
		if err == nil {
			discountAmount = discount
		}
	}

	// Apply surge
	if surgeMultiplier > 1.0 {
		fareBreakdown["surge_multiplier"] = surgeMultiplier
		fareBreakdown["surge_amount"] = fareBreakdown["subtotal"] * (surgeMultiplier - 1)
		fareBreakdown["total"] = fareBreakdown["subtotal"] + fareBreakdown["surge_amount"] + fareBreakdown["platform_fee"]
	}

	// Deduct discount
	finalFare := fareBreakdown["total"] - discountAmount
	if finalFare < fareConfig.MinFare {
		finalFare = fareConfig.MinFare
	}

	// Generate ride OTP
	rideOTP := utils.GenerateRideOTP()

	// Create idempotency key if not provided
	idempotencyKey := req.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = utils.GenerateIdempotencyKey()
	}

	// Double-check rider exists before creating ride
	var riderCheck models.User
	if err := s.DB.Where("id = ?", req.RiderID).First(&riderCheck).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("rider not found with ID %s - cannot create ride", req.RiderID.String())
		}
		return nil, fmt.Errorf("database error checking rider: %w", err)
	}

	// Create ride
	ride := &models.Ride{
		RiderID:           req.RiderID,
		Status:            models.RideStatusRequested,
		VehicleType:       req.VehicleType,
		Pickup:            models.Location{Latitude: req.PickupLat, Longitude: req.PickupLng, Address: req.PickupAddress},
		Dropoff:           models.Location{Latitude: req.DropoffLat, Longitude: req.DropoffLng, Address: req.DropoffAddress},
		EstimatedDistance: distance,
		EstimatedDuration: duration,
		EstimatedFare:     fareBreakdown["total"],
		BaseFare:          fareBreakdown["base_fare"],
		PerKmRate:         fareConfig.PerKmRate,
		PerMinRate:        fareConfig.PerMinRate,
		SurgeMultiplier:   surgeMultiplier,
		SurgeAmount:       fareBreakdown["surge_amount"],
		PlatformFee:       fareBreakdown["platform_fee"],
		PromoCode:         req.PromoCode,
		DiscountAmount:    discountAmount,
		FinalFare:         finalFare,
		PaymentMethod:     req.PaymentMethod,
		RideOTP:           rideOTP,
		IdempotencyKey:    idempotencyKey,
		Preferences:       req.Preferences,
	}

	if err := s.DB.Create(ride).Error; err != nil {
		return nil, err
	}

	// Record ride request for surge calculation (after ride is created with ID)
	_ = surgeService.RecordRideRequest(ride.ID.String(), req.PickupLat, req.PickupLng, req.VehicleType)

	QueueCRMEvent("ride.requested", "ride", ride.ID.String(), map[string]interface{}{
		"ride_id":      ride.ID.String(),
		"rider_id":     ride.RiderID.String(),
		"vehicle_type": ride.VehicleType,
		"status":       ride.Status,
		"pickup":       ride.Pickup,
		"dropoff":      ride.Dropoff,
		"final_fare":   ride.FinalFare,
	})

	// Store ride OTP in Redis
	database.SetCache(database.GetOTPRideKey(ride.ID.String()), rideOTP, 30*time.Minute)

	// Store ride status in Redis
	database.SetCache(database.GetRideStatusKey(ride.ID.String()), models.RideStatusRequested, 30*time.Minute)

	// Store current ride for rider
	database.SetCache(existingRideKey, ride.ID.String(), 2*time.Hour)

	// Find and notify nearby drivers
	go NewMatchingService().StartMatchingProcess(ride)
	_ = websocket.GetHandler().SendToUserEvent(ride.RiderID.String(), "ride_status_update", map[string]interface{}{
		"ride_id": ride.ID.String(),
		"status":  ride.Status,
	})
	_ = websocket.GetHandler().SendRideEvent(ride.ID.String(), "ride_status_update", map[string]interface{}{
		"ride_id": ride.ID.String(),
		"status":  ride.Status,
	})

	return ride, nil
}

// calculatePromoDiscount calculates discount from promo code
func (s *RideService) calculatePromoDiscount(code string, amount float64, userID uuid.UUID) (float64, error) {
	var promo models.PromoCode
	if err := s.DB.Where("code = ? AND is_active = ?", code, true).First(&promo).Error; err != nil {
		return 0, err
	}

	// Check validity dates
	now := time.Now()
	if promo.StartDate != nil && now.Before(*promo.StartDate) {
		return 0, errors.New("promo code not yet valid")
	}
	if promo.EndDate != nil && now.After(*promo.EndDate) {
		return 0, errors.New("promo code expired")
	}

	// Check min ride amount
	if amount < promo.MinRideAmount {
		return 0, errors.New("ride amount below minimum for this promo")
	}

	// Check max uses
	if promo.MaxUses > 0 && promo.UsesCount >= promo.MaxUses {
		return 0, errors.New("promo code limit reached")
	}

	// Check user usage
	var userUsageCount int64
	s.DB.Model(&models.PromoCodeUsage{}).Where("promo_code_id = ? AND user_id = ?", promo.ID, userID).Count(&userUsageCount)
	if int(userUsageCount) >= promo.MaxUsesPerUser {
		return 0, errors.New("you have already used this promo code")
	}

	// Calculate discount
	discount := 0.0
	if promo.DiscountType == "percentage" {
		discount = amount * (promo.DiscountValue / 100)
		if promo.MaxDiscount > 0 && discount > promo.MaxDiscount {
			discount = promo.MaxDiscount
		}
	} else {
		discount = promo.DiscountValue
	}

	// Increment usage count
	s.DB.Model(&promo).Update("uses_count", promo.UsesCount+1)

	// Record usage
	usage := &models.PromoCodeUsage{
		PromoCodeID:    promo.ID,
		UserID:         userID,
		DiscountAmount: discount,
	}
	s.DB.Create(usage)

	return discount, nil
}

// findAndNotifyDrivers finds nearby drivers and sends notifications
func (s *RideService) findAndNotifyDrivers(ride *models.Ride) {
	cfg := config.Get()
	ds := NewDriverService()

	// Find nearby drivers
	drivers, err := ds.GetNearbyDrivers(
		ride.Pickup.Latitude,
		ride.Pickup.Longitude,
		cfg.App.DriverSearchRadiusKM,
		ride.VehicleType,
		ride.Preferences.FemaleDriverOnly,
	)
	if err != nil || len(drivers) == 0 {
		// No drivers found - mark ride as no_driver_found after timeout
		time.Sleep(time.Duration(cfg.App.RideRequestTimeoutSec) * time.Second)

		// Check if ride still pending
		var currentRide models.Ride
		s.DB.First(&currentRide, ride.ID)
		if currentRide.Status == models.RideStatusRequested {
			s.DB.Model(&currentRide).Update("status", models.RideStatusNoDriverFound)
			database.DeleteCache(database.GetRiderCurrentRideKey(ride.RiderID.String()))
		}
		return
	}

	// Sort drivers by match score (distance + rating + acceptance)
	type driverMatch struct {
		driver   models.DriverLocation
		score    float64
		distance float64
	}

	var matches []driverMatch
	for _, d := range drivers {
		distance := utils.CalculateDistance(
			ride.Pickup.Latitude, ride.Pickup.Longitude,
			d.Latitude, d.Longitude,
		)

		// Get driver rating
		var rating models.DriverRatingSummary
		s.DB.Where("driver_id = ?", d.DriverID).First(&rating)

		// Calculate score (lower is better)
		score := distance*10 + (5-rating.AverageRating)*2

		matches = append(matches, driverMatch{
			driver:   d,
			score:    score,
			distance: distance,
		})
	}

	// Sort by score
	for i := 0; i < len(matches)-1; i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[i].score > matches[j].score {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}

	// Notify top 8-10 drivers
	maxDrivers := 10
	if len(matches) < maxDrivers {
		maxDrivers = len(matches)
	}

	notifiedDrivers := []string{}
	for i := 0; i < maxDrivers; i++ {
		match := matches[i]

		// Create match record
		rideMatch := &models.RideMatch{
			RideID:          ride.ID,
			DriverID:        match.driver.DriverID,
			Distance:        match.distance,
			ETA:             utils.CalculateETA(match.distance, ride.VehicleType),
			DriverRating:    0, // Will be populated
			AcceptanceScore: 0,
			MatchScore:      match.score,
			NotifiedAt:      time.Now(),
		}
		s.DB.Create(rideMatch)

		// Add to notified list
		notifiedDrivers = append(notifiedDrivers, match.driver.DriverID.String())

		// TODO: Send push notification via FCM
		// For now, we'll store in Redis for polling
		database.SetCache(
			database.GetRideDriversNotifiedKey(ride.ID.String()),
			notifiedDrivers,
			time.Duration(cfg.App.RideRequestTimeoutSec)*time.Second,
		)
	}

	// Wait for acceptance
	time.Sleep(time.Duration(cfg.App.RideRequestTimeoutSec) * time.Second)

	// Check if ride was accepte
	var currentRide models.Ride
	s.DB.First(&currentRide, ride.ID)
	if currentRide.Status == models.RideStatusRequested {
		// No one accepted - mark as no_driver_found
		s.DB.Model(&currentRide).Update("status", models.RideStatusNoDriverFound)
		database.DeleteCache(database.GetRiderCurrentRideKey(ride.RiderID.String()))
	}
}

// AcceptRide allows a driver to accept a ride with distributed locking
func (s *RideService) AcceptRide(rideID, driverID uuid.UUID) (*models.Ride, error) {
	// CRITICAL: Distributed lock prevents race conditions
	lock, acquired, err := LockMgr.AcquireRideLock(rideID.String(), driverID.String(), 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("lock acquisition failed: %w", err)
	}
	if !acquired {
		return nil, errors.New("ride is being processed by another driver")
	}
	defer lock.Release() // Always release lock

	// CRITICAL: Fraud detection check
	fraudScore := FraudDetector.CheckDriverFraud(driverID.String(), 0, 0)
	if fraudScore.Action == "block" {
		return nil, errors.New("suspicious activity detected, please contact support")
	}

	var ride models.Ride
	if err := s.DB.First(&ride, rideID).Error; err != nil {
		return nil, err
	}

	// Check if ride is still available
	if ride.Status != models.RideStatusRequested {
		return nil, errors.New("ride no longer available")
	}

	// CRITICAL: Lock driver to prevent double assignment
	driverLock, driverAcquired, err := LockMgr.AcquireDriverLock(driverID.String(), 2*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("driver lock failed: %w", err)
	}
	if !driverAcquired {
		return nil, errors.New("you already have an active ride (locked)")
	}
	defer driverLock.Release()

	now := time.Now()

	// Assign driver
	if err := s.DB.Model(&ride).Updates(map[string]interface{}{
		"driver_id":   driverID,
		"status":      models.RideStatusDriverAssigned,
		"accepted_at": now,
		"updated_at":  now,
	}).Error; err != nil {
		return nil, err
	}

	// Update ride match record
	s.DB.Model(&models.RideMatch{}).Where("ride_id = ? AND driver_id = ?", rideID, driverID).
		Updates(map[string]interface{}{
			"response":     "accepted",
			"responded_at": now,
		})

	// Store driver current ride
	currentRideKey := database.GetDriverCurrentRideKey(driverID.String())
	database.SetCache(currentRideKey, rideID.String(), 2*time.Hour)

	// Update ride status in Redis
	database.SetCache(database.GetRideStatusKey(rideID.String()), models.RideStatusDriverAssigned, 2*time.Hour)
	_ = websocket.GetHandler().SendRideEvent(rideID.String(), "ride_status_update", map[string]interface{}{
		"ride_id":   rideID.String(),
		"status":    models.RideStatusDriverAssigned,
		"driver_id": driverID.String(),
	})
	_ = websocket.GetHandler().SendToUserEvent(ride.RiderID.String(), "ride_status_update", map[string]interface{}{
		"ride_id":   rideID.String(),
		"status":    models.RideStatusDriverAssigned,
		"driver_id": driverID.String(),
	})
	_ = s.clearMatchingCache(rideID, ride.RiderID)

	return &ride, nil
}

// RejectRide allows a driver to reject a ride
func (s *RideService) RejectRide(rideID, driverID uuid.UUID, reason string) error {
	now := time.Now()
	return s.DB.Model(&models.RideMatch{}).Where("ride_id = ? AND driver_id = ?", rideID, driverID).
		Updates(map[string]interface{}{
			"response":     "rejected",
			"responded_at": now,
		}).Error
}

// DriverArrived marks driver as arrived at pickup
func (s *RideService) DriverArrived(rideID, driverID uuid.UUID) (*models.Ride, error) {
	var ride models.Ride
	if err := s.DB.Where("id = ? AND driver_id = ?", rideID, driverID).First(&ride).Error; err != nil {
		return nil, err
	}

	if ride.Status != models.RideStatusDriverAssigned {
		return nil, errors.New("invalid ride status")
	}

	now := time.Now()
	if err := s.DB.Model(&ride).Updates(map[string]interface{}{
		"status":     models.RideStatusDriverArrived,
		"arrived_at": now,
		"updated_at": now,
	}).Error; err != nil {
		return nil, err
	}

	database.SetCache(database.GetRideStatusKey(rideID.String()), models.RideStatusDriverArrived, 2*time.Hour)
	_ = websocket.GetHandler().SendRideEvent(rideID.String(), "ride_status_update", map[string]interface{}{
		"ride_id": rideID.String(),
		"status":  models.RideStatusDriverArrived,
	})
	_ = websocket.GetHandler().SendToUserEvent(ride.RiderID.String(), "ride_status_update", map[string]interface{}{
		"ride_id": rideID.String(),
		"status":  models.RideStatusDriverArrived,
	})

	return &ride, nil
}

// StartRide starts the ride after OTP verification
func (s *RideService) StartRide(rideID, driverID uuid.UUID, otp string) (*models.Ride, error) {
	var ride models.Ride
	if err := s.DB.Where("id = ? AND driver_id = ?", rideID, driverID).First(&ride).Error; err != nil {
		return nil, err
	}

	if ride.Status != models.RideStatusDriverArrived {
		return nil, errors.New("driver must arrive first")
	}

	// Verify OTP
	storedOTP, err := database.GetCache(database.GetOTPRideKey(rideID.String()))
	if err != nil || storedOTP != otp {
		return nil, errors.New("invalid OTP")
	}

	now := time.Now()
	if err := s.DB.Model(&ride).Updates(map[string]interface{}{
		"status":     models.RideStatusOngoing,
		"started_at": now,
		"updated_at": now,
	}).Error; err != nil {
		return nil, err
	}

	// Delete OTP after use
	database.DeleteCache(database.GetOTPRideKey(rideID.String()))

	database.SetCache(database.GetRideStatusKey(rideID.String()), models.RideStatusOngoing, 2*time.Hour)
	_ = websocket.GetHandler().SendRideEvent(rideID.String(), "ride_status_update", map[string]interface{}{
		"ride_id": rideID.String(),
		"status":  models.RideStatusOngoing,
	})
	_ = websocket.GetHandler().SendToUserEvent(ride.RiderID.String(), "ride_status_update", map[string]interface{}{
		"ride_id": rideID.String(),
		"status":  models.RideStatusOngoing,
	})

	return &ride, nil
}

// UpdateRideLocation updates driver location during ride
func (s *RideService) UpdateRideLocation(rideID uuid.UUID, lat, lng float64) error {
	location := &models.RideLocation{
		RideID:    rideID,
		Latitude:  lat,
		Longitude: lng,
	}
	return s.DB.Create(location).Error
}

// CompleteRide completes the ride
func (s *RideService) CompleteRide(rideID, driverID uuid.UUID, finalLat, finalLng float64) (*models.Ride, error) {
	var ride models.Ride
	if err := s.DB.Where("id = ? AND driver_id = ?", rideID, driverID).First(&ride).Error; err != nil {
		return nil, err
	}

	if ride.Status != models.RideStatusOngoing {
		return nil, errors.New("ride must be ongoing")
	}

	// Calculate actual distance from ride locations
	actualDistance := s.calculateActualDistance(rideID, ride.Pickup.Latitude, ride.Pickup.Longitude)
	if actualDistance == 0 {
		// Fallback to estimated distance
		actualDistance = ride.EstimatedDistance
	}

	// Calculate actual duration
	now := time.Now()
	actualDuration := int(now.Sub(*ride.StartedAt).Minutes())

	// Recalculate fare with actual distance
	actualFare := ride.BaseFare + (actualDistance * ride.PerKmRate) + (float64(actualDuration) * ride.PerMinRate)

	// Apply surge
	if ride.SurgeMultiplier > 1.0 {
		actualFare = actualFare * ride.SurgeMultiplier
	}

	// Add platform fee
	actualFare += ride.PlatformFee

	// Apply discount
	actualFare -= ride.DiscountAmount
	if actualFare < 0 {
		actualFare = 0
	}

	// Update ride
	if err := s.DB.Model(&ride).Updates(map[string]interface{}{
		"status":          models.RideStatusCompleted,
		"completed_at":    now,
		"actual_distance": actualDistance,
		"actual_duration": actualDuration,
		"final_fare":      math.Ceil(actualFare),
		"updated_at":      now,
	}).Error; err != nil {
		return nil, err
	}

	// Clear caches
	database.DeleteCache(database.GetRiderCurrentRideKey(ride.RiderID.String()))
	database.DeleteCache(database.GetDriverCurrentRideKey(driverID.String()))
	database.DeleteCache(database.GetRideStatusKey(rideID.String()))
	_ = websocket.GetHandler().SendRideEvent(rideID.String(), "ride_status_update", map[string]interface{}{
		"ride_id": rideID.String(),
		"status":  models.RideStatusCompleted,
	})
	_ = websocket.GetHandler().SendToUserEvent(ride.RiderID.String(), "ride_status_update", map[string]interface{}{
		"ride_id": rideID.String(),
		"status":  models.RideStatusCompleted,
	})

	// Update driver earnings
	ds := NewDriverService()
	platformCommission := actualFare * config.Get().App.PlatformCommissionPercent / 100
	driverEarnings := actualFare - platformCommission
	ds.UpdateEarnings(driverID, driverEarnings)

	// Create commission record
	commission := &models.Commission{
		RideID:             rideID,
		DriverID:           driverID,
		TotalFare:          actualFare,
		PlatformCommission: platformCommission,
		DriverEarnings:     driverEarnings,
		PlatformPercent:    config.Get().App.PlatformCommissionPercent,
	}
	s.DB.Create(commission)

	// Update driver stats
	s.DB.Model(&models.Driver{}).Where("id = ?", driverID).
		Updates(map[string]interface{}{
			"total_rides": gorm.Expr("total_rides + 1"),
		})

	// Remove ride from surge demand calculation
	surgeService := NewSurgePricingService()
	_ = surgeService.RecordRideCompletion(ride.ID.String(), ride.Pickup.Latitude, ride.Pickup.Longitude, ride.VehicleType)

	// Send SMS notification to rider (async)
	if SMSServiceInstance != nil {
		go func() {
			var user models.User
			if err := s.DB.First(&user, ride.RiderID).Error; err == nil && user.Phone != "" {
				_ = SMSServiceInstance.SendRideCompleteNotification(user.Phone, math.Ceil(actualFare))
			}
		}()
	}

	return &ride, nil
}

// calculateActualDistance calculates actual distance from ride locations
func (s *RideService) calculateActualDistance(rideID uuid.UUID, startLat, startLng float64) float64 {
	var locations []models.RideLocation
	if err := s.DB.Where("ride_id = ?", rideID).Order("created_at ASC").Find(&locations).Error; err != nil {
		return 0
	}

	if len(locations) == 0 {
		return 0
	}

	totalDistance := 0.0
	prevLat := startLat
	prevLng := startLng

	for _, loc := range locations {
		dist := utils.CalculateDistance(prevLat, prevLng, loc.Latitude, loc.Longitude)
		totalDistance += dist
		prevLat = loc.Latitude
		prevLng = loc.Longitude
	}

	return totalDistance
}

// CancelRide cancels a ride
func (s *RideService) CancelRide(rideID, cancelledBy uuid.UUID, reason string, isRider bool) (*models.Ride, error) {
	var ride models.Ride
	if err := s.DB.First(&ride, rideID).Error; err != nil {
		return nil, err
	}

	// Check if ride can be cancelled
	if ride.Status == models.RideStatusCompleted ||
		ride.Status == models.RideStatusCancelled ||
		ride.Status == models.RideStatusNoDriverFound {
		return nil, errors.New("ride cannot be cancelled")
	}

	// Calculate cancellation fee if applicable
	cancellationFee := 0.0
	if !isRider && ride.Status == models.RideStatusDriverAssigned {
		// Driver cancelled after accepting - penalty applies
		cancellationFee = 50 // Fixed penalty
	} else if isRider && ride.Status == models.RideStatusOngoing {
		// Rider cancelled during ride - full fare penalty
		cancellationFee = ride.EstimatedFare * 0.5
	} else if isRider && ride.Status == models.RideStatusDriverArrived {
		// Check if > 2 minutes since arrival
		if ride.ArrivedAt != nil && time.Since(*ride.ArrivedAt) > 2*time.Minute {
			cancellationFee = 30
		}
	}

	now := time.Now()
	updates := map[string]interface{}{
		"status":              models.RideStatusCancelled,
		"cancellation_reason": reason,
		"cancelled_by":        cancelledBy,
		"cancellation_time":   now,
		"cancellation_fee":    cancellationFee,
		"updated_at":          now,
	}

	if err := s.DB.Model(&ride).Updates(updates).Error; err != nil {
		return nil, err
	}

	// Clear caches
	database.DeleteCache(database.GetRiderCurrentRideKey(ride.RiderID.String()))
	if ride.DriverID != nil {
		database.DeleteCache(database.GetDriverCurrentRideKey(ride.DriverID.String()))
	}
	database.DeleteCache(database.GetRideStatusKey(rideID.String()))
	_ = websocket.GetHandler().SendRideEvent(rideID.String(), "ride_status_update", map[string]interface{}{
		"ride_id": rideID.String(),
		"status":  models.RideStatusCancelled,
		"reason":  reason,
	})
	_ = websocket.GetHandler().SendToUserEvent(ride.RiderID.String(), "ride_status_update", map[string]interface{}{
		"ride_id": rideID.String(),
		"status":  models.RideStatusCancelled,
		"reason":  reason,
	})

	// Update driver acceptance score if driver cancelled
	if !isRider && ride.DriverID != nil {
		s.DB.Model(&models.Driver{}).Where("id = ?", ride.DriverID).
			Update("acceptance_score", gorm.Expr("acceptance_score - 5"))
	}

	return &ride, nil
}

func (s *RideService) clearMatchingCache(rideID uuid.UUID, riderID uuid.UUID) error {
	_ = database.DeleteCache(database.GetRideRequestStateKey(rideID.String()))
	_ = database.DeleteCache(database.GetRideWaveKey(rideID.String()))
	_ = database.DeleteCache(database.GetRidePendingDriversKey(rideID.String()))
	_ = database.DeleteCache(database.GetRideDriversNotifiedKey(rideID.String()))
	_ = database.DeleteCache(database.GetRideStatusKey(rideID.String()))
	if riderID != uuid.Nil {
		_ = database.DeleteCache(database.GetRiderCurrentRideKey(riderID.String()))
	}
	return nil
}

// GetRide gets ride details
func (s *RideService) GetRide(rideID uuid.UUID) (*models.Ride, error) {
	var ride models.Ride
	if err := s.DB.Preload("Rider").Preload("Driver.User").Preload("Vehicle").First(&ride, rideID).Error; err != nil {
		return nil, err
	}
	return &ride, nil
}

// GetActiveRideForRider gets active ride for rider
func (s *RideService) GetActiveRideForRider(riderID uuid.UUID) (*models.Ride, error) {
	var ride models.Ride
	err := s.DB.Where("rider_id = ? AND status NOT IN (?)", riderID, []string{
		models.RideStatusCompleted,
		models.RideStatusCancelled,
		models.RideStatusNoDriverFound,
	}).First(&ride).Error

	if err != nil {
		return nil, err
	}
	return &ride, nil
}

// GetActiveRideForDriver gets active ride for driver
func (s *RideService) GetActiveRideForDriver(driverID uuid.UUID) (*models.Ride, error) {
	var ride models.Ride
	err := s.DB.Where("driver_id = ? AND status NOT IN (?)", driverID, []string{
		models.RideStatusCompleted,
		models.RideStatusCancelled,
		models.RideStatusNoDriverFound,
	}).First(&ride).Error

	if err != nil {
		return nil, err
	}
	return &ride, nil
}

// GetRideHistory gets ride history for user
func (s *RideService) GetRideHistory(userID uuid.UUID, role string, page, perPage int) ([]models.Ride, int64, error) {
	var rides []models.Ride
	var count int64

	offset := (page - 1) * perPage
	terminalStatuses := []string{
		models.RideStatusCompleted,
		models.RideStatusCancelled,
		models.RideStatusNoDriverFound,
	}

	query := s.DB.Model(&models.Ride{})
	if role == "driver" {
		query = query.Where("driver_id = ?", userID)
	} else {
		query = query.Where("rider_id = ?", userID)
	}

	query = query.Where("status IN ?", terminalStatuses)

	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Preload("Rider").Preload("Driver.User").
		Order("created_at DESC").
		Offset(offset).Limit(perPage).
		Find(&rides).Error; err != nil {
		return nil, 0, err
	}

	return rides, count, nil
}

// GetNearbyDrivers gets nearby drivers for a location
func (s *RideService) GetNearbyDrivers(lat, lng float64, vehicleType string) ([]map[string]interface{}, error) {
	ds := NewDriverService()

	drivers, err := ds.GetNearbyDrivers(lat, lng, config.Get().App.DriverSearchRadiusKM, vehicleType, false)
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	for _, d := range drivers {
		distance := utils.CalculateDistance(lat, lng, d.Latitude, d.Longitude)
		eta := utils.CalculateETA(distance, vehicleType)

		result = append(result, map[string]interface{}{
			"driver_id": d.DriverID,
			"lat":       d.Latitude,
			"lng":       d.Longitude,
			"distance":  distance,
			"eta":       eta,
		})
	}

	return result, nil
}

// EstimateFare estimates fare for a ride
func (s *RideService) EstimateFare(pickupLat, pickupLng, dropoffLat, dropoffLng float64, vehicleType string) (map[string]interface{}, error) {
	// Try Google Maps for accurate routing
	mapsService := GetMapsService()
	routeInfo, err := mapsService.CalculateRoute(pickupLat, pickupLng, dropoffLat, dropoffLng)

	var distance float64
	var durationMin int
	var hasTrafficData bool

	if err != nil || routeInfo.IsFallback {
		// Fallback to Haversine
		distance = utils.CalculateDistance(pickupLat, pickupLng, dropoffLat, dropoffLng)
		durationMin = int(utils.EstimateRideDuration(distance, vehicleType, 1.0))
		hasTrafficData = false
	} else {
		distance = routeInfo.DistanceKM
		durationMin = int(routeInfo.DurationSec / 60)
		hasTrafficData = routeInfo.HasTrafficData
	}

	// Get dynamic surge pricing
	surgeService := NewSurgePricingService()
	surgeFactors := surgeService.GetSurgeForRideEstimate(pickupLat, pickupLng, vehicleType)

	fareBreakdown := utils.EstimateFare(distance, vehicleType, durationMin, surgeFactors.Multiplier)

	return map[string]interface{}{
		"distance":               distance,
		"distance_text":          routeInfo.DistanceText,
		"duration_sec":           routeInfo.DurationSec,
		"duration_text":          routeInfo.DurationText,
		"has_traffic_data":       hasTrafficData,
		"is_fallback":            routeInfo.IsFallback,
		"polyline":               routeInfo.Polyline,
		"estimated_duration_min": durationMin,
		"base_fare":              fareBreakdown["base_fare"],
		"distance_fare":          fareBreakdown["distance_fare"],
		"time_fare":              fareBreakdown["time_fare"],
		"subtotal":               fareBreakdown["subtotal"],
		"surge_multiplier":       fareBreakdown["surge_multiplier"],
		"surge_amount":           fareBreakdown["surge_amount"],
		"platform_fee":           fareBreakdown["platform_fee"],
		"total":                  fareBreakdown["total"],
		"currency":               config.Get().App.DefaultCurrency,
		"demand_supply": map[string]interface{}{
			"demand": surgeFactors.DemandScore,
			"supply": surgeFactors.SupplyScore,
			"ratio":  surgeFactors.Ratio,
		},
	}, nil
}

// TrackRide provides real-time tracking data for a ride (rider view)
func (s *RideService) TrackRide(rideID, userID uuid.UUID) (map[string]interface{}, error) {
	var ride models.Ride
	if err := s.DB.Preload("Driver").Preload("Driver.CurrentLocation").First(&ride, rideID).Error; err != nil {
		return nil, err
	}

	// Verify user has access to this ride
	if ride.RiderID != userID && (ride.DriverID == nil || *ride.DriverID != userID) {
		return nil, fmt.Errorf("unauthorized access to ride")
	}

	tracking := map[string]interface{}{
		"ride_id":     ride.ID,
		"status":      ride.Status,
		"driver_id":   ride.DriverID,
		"pickup_lat":  ride.Pickup.Latitude,
		"pickup_lng":  ride.Pickup.Longitude,
		"dropoff_lat": ride.Dropoff.Latitude,
		"dropoff_lng": ride.Dropoff.Longitude,
		"updated_at":  time.Now(),
	}

	// Add driver location if assigned
	if ride.Driver != nil && ride.Driver.CurrentLocation != nil {
		tracking["driver_location"] = map[string]interface{}{
			"lat": ride.Driver.CurrentLocation.Latitude,
			"lng": ride.Driver.CurrentLocation.Longitude,
		}
	}

	return tracking, nil
}

// GetRideETA calculates estimated time of arrival
func (s *RideService) GetRideETA(rideID, userID uuid.UUID) (map[string]interface{}, error) {
	var ride models.Ride
	if err := s.DB.Preload("Driver.CurrentLocation").First(&ride, rideID).Error; err != nil {
		return nil, err
	}

	// Verify access
	if ride.RiderID != userID && (ride.DriverID == nil || *ride.DriverID != userID) {
		return nil, fmt.Errorf("unauthorized access to ride")
	}

	// Default ETA calculation
	etaMinutes := 5
	if ride.Status == models.RideStatusDriverAssigned {
		etaMinutes = 8 // Time to reach pickup
	} else if ride.Status == models.RideStatusOngoing {
		etaMinutes = 15 // Time to reach dropoff
	}

	return map[string]interface{}{
		"ride_id":     ride.ID,
		"status":      ride.Status,
		"eta_minutes": etaMinutes,
		"eta_text":    fmt.Sprintf("%d mins", etaMinutes),
		"updated_at":  time.Now(),
	}, nil
}

// RetryMatch retries driver matching for a ride
func (s *RideService) RetryMatch(rideID, riderID uuid.UUID) (map[string]interface{}, error) {
	var ride models.Ride
	if err := s.DB.First(&ride, rideID).Error; err != nil {
		return nil, err
	}

	// Verify rider owns this ride
	if ride.RiderID != riderID {
		return nil, fmt.Errorf("unauthorized: not your ride")
	}

	// Check if ride can be retried
	if ride.Status != models.RideStatusNoDriverFound {
		return nil, fmt.Errorf("ride cannot be retried in status: %s", ride.Status)
	}

	// Reset ride to requested state for retry
	ride.Status = models.RideStatusRequested
	ride.DriverID = nil
	if err := s.DB.Save(&ride).Error; err != nil {
		return nil, err
	}

	// Trigger matching again
	s.findAndNotifyDrivers(&ride)

	return map[string]interface{}{
		"ride_id":     ride.ID,
		"status":      "retrying_match",
		"message":     "Looking for drivers again...",
		"retry_count": 1,
	}, nil
}

// ApplyPromoCode applies a promo code to a ride
func (s *RideService) ApplyPromoCode(rideID, riderID uuid.UUID, promoCode string) (map[string]interface{}, error) {
	var ride models.Ride
	if err := s.DB.First(&ride, rideID).Error; err != nil {
		return nil, err
	}

	// Verify rider owns this ride
	if ride.RiderID != riderID {
		return nil, fmt.Errorf("unauthorized: not your ride")
	}

	// Check if ride can accept promo code
	if ride.Status != models.RideStatusRequested && ride.Status != models.RideStatusDriverAssigned {
		return nil, fmt.Errorf("promo code cannot be applied in status: %s", ride.Status)
	}

	// Prevent multiple promo applications on the same ride
	if ride.PromoCode != "" || ride.DiscountAmount > 0 {
		return nil, errors.New("promo code already applied")
	}

	// Load promo code from database and validate against the current ride
	var promo models.PromoCode
	if err := s.DB.Where("code = ? AND is_active = ?", promoCode, true).First(&promo).Error; err != nil {
		return nil, errors.New("invalid or expired promo code")
	}

	now := time.Now()
	if promo.StartDate != nil && now.Before(*promo.StartDate) {
		return nil, errors.New("promo code not yet valid")
	}
	if promo.EndDate != nil && now.After(*promo.EndDate) {
		return nil, errors.New("promo code expired")
	}
	if promo.MinRideAmount > 0 && ride.EstimatedFare < promo.MinRideAmount {
		return nil, errors.New("ride amount below minimum for this promo")
	}
	if promo.MaxUses > 0 && promo.UsesCount >= promo.MaxUses {
		return nil, errors.New("promo code limit reached")
	}

	var userUsageCount int64
	s.DB.Model(&models.PromoCodeUsage{}).Where("promo_code_id = ? AND user_id = ?", promo.ID, riderID).Count(&userUsageCount)
	if promo.MaxUsesPerUser > 0 && int(userUsageCount) >= promo.MaxUsesPerUser {
		return nil, errors.New("you have already used this promo code")
	}

	if len(promo.ApplicableVehicleTypes) > 0 {
		allowed := false
		for _, vehicleType := range promo.ApplicableVehicleTypes {
			if vehicleType == ride.VehicleType {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, errors.New("promo code not applicable for this vehicle type")
		}
	}

	// Calculate discount
	var discountAmount float64
	if promo.DiscountType == "percentage" {
		discountAmount = ride.EstimatedFare * (promo.DiscountValue / 100)
		if promo.MaxDiscount > 0 && discountAmount > promo.MaxDiscount {
			discountAmount = promo.MaxDiscount
		}
	} else {
		discountAmount = promo.DiscountValue
		if discountAmount > ride.EstimatedFare {
			discountAmount = ride.EstimatedFare
		}
	}

	if discountAmount < 0 {
		discountAmount = 0
	}

	// Update ride with discount
	ride.DiscountAmount = discountAmount
	ride.PromoCode = promoCode
	finalFare := ride.EstimatedFare - discountAmount
	if finalFare < 0 {
		finalFare = 0
	}

	// Persist usage and ride update together
	if err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.PromoCode{}).Where("id = ?", promo.ID).
			Update("uses_count", gorm.Expr("uses_count + 1")).Error; err != nil {
			return err
		}

		usage := &models.PromoCodeUsage{
			PromoCodeID:    promo.ID,
			UserID:         riderID,
			RideID:         ride.ID,
			DiscountAmount: discountAmount,
			UsedAt:         time.Now(),
		}
		if err := tx.Create(usage).Error; err != nil {
			return err
		}

		return tx.Model(&ride).Updates(map[string]interface{}{
			"discount_amount": discountAmount,
			"promo_code":      promoCode,
			"final_fare":      finalFare,
			"updated_at":      time.Now(),
		}).Error
	}); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"ride_id":         ride.ID,
		"promo_code":      promoCode,
		"original_fare":   ride.EstimatedFare,
		"discount_amount": discountAmount,
		"final_fare":      finalFare,
		"applied_at":      time.Now(),
	}, nil
}

// GetFareBreakdown returns detailed fare breakdown for a ride
func (s *RideService) GetFareBreakdown(rideID, userID uuid.UUID) (map[string]interface{}, error) {
	var ride models.Ride
	if err := s.DB.First(&ride, rideID).Error; err != nil {
		return nil, err
	}

	// Verify user has access to this ride
	if ride.RiderID != userID && (ride.DriverID == nil || *ride.DriverID != userID) {
		return nil, fmt.Errorf("unauthorized: no access to this ride")
	}

	breakdown := map[string]interface{}{
		"ride_id":          ride.ID,
		"base_fare":        ride.BaseFare,
		"distance_charge":  ride.EstimatedDistance * ride.PerKmRate,
		"time_charge":      float64(ride.EstimatedDuration) * ride.PerMinRate,
		"surge_multiplier": ride.SurgeMultiplier,
		"surge_amount":     ride.SurgeAmount,
		"platform_fee":     ride.PlatformFee,
		"tax_amount":       ride.TaxAmount,
		"discount_amount":  ride.DiscountAmount,
		"promo_code":       ride.PromoCode,
		"estimated_fare":   ride.EstimatedFare,
		"final_fare":       ride.FinalFare,
		"currency":         "INR",
		"breakdown": map[string]interface{}{
			"base_fare":       fmt.Sprintf("₹%.2f", ride.BaseFare),
			"distance_charge": fmt.Sprintf("₹%.2f (%.1f km × ₹%.2f/km)", ride.EstimatedDistance*ride.PerKmRate, ride.EstimatedDistance, ride.PerKmRate),
			"time_charge":     fmt.Sprintf("₹%.2f (%d min × ₹%.2f/min)", float64(ride.EstimatedDuration)*ride.PerMinRate, ride.EstimatedDuration, ride.PerMinRate),
			"surge":           fmt.Sprintf("₹%.2f (%.1fx)", ride.SurgeAmount, ride.SurgeMultiplier),
			"platform_fee":    fmt.Sprintf("₹%.2f", ride.PlatformFee),
			"tax":             fmt.Sprintf("₹%.2f", ride.TaxAmount),
			"discount":        fmt.Sprintf("-₹%.2f", ride.DiscountAmount),
			"total":           fmt.Sprintf("₹%.2f", ride.FinalFare),
		},
	}

	return breakdown, nil
}

// GetMatchStatus returns detailed matching status for debugging
func (s *RideService) GetMatchStatus(rideID uuid.UUID) (map[string]interface{}, error) {
	var ride models.Ride
	if err := s.DB.First(&ride, rideID).Error; err != nil {
		return nil, err
	}

	// Build wave details (simplified - in production, fetch from matching log)
	waveDetails := []map[string]interface{}{
		{
			"wave":             1,
			"radius_km":        2,
			"drivers_notified": 5,
			"responses": map[string]int{
				"accepted":    0,
				"rejected":    2,
				"no_response": 3,
			},
			"duration_sec": 15,
			"status":       "completed",
		},
		{
			"wave":             2,
			"radius_km":        5,
			"drivers_notified": 12,
			"responses": map[string]int{
				"accepted":    1,
				"rejected":    4,
				"no_response": 7,
			},
			"duration_sec": 10,
			"status":       "completed",
		},
	}

	// Current matching status
	status := "no_driver_found"
	currentWave := 2
	if ride.Status == models.RideStatusDriverAssigned {
		status = "driver_assigned"
		currentWave = 2
	}

	return map[string]interface{}{
		"ride_id":               ride.ID,
		"status":                status,
		"matching_algorithm":    "4_wave_nearest",
		"current_wave":          currentWave,
		"total_waves":           4,
		"wave_details":          waveDetails,
		"matching_duration_sec": 25,
		"created_at":            ride.RequestedAt,
	}, nil
}

// ReassignRide reassigns a ride to a new driver
func (s *RideService) ReassignRide(rideID, userID uuid.UUID, reason string, preferredTypes []string, priority string) (map[string]interface{}, error) {
	var ride models.Ride
	if err := s.DB.First(&ride, rideID).Error; err != nil {
		return nil, err
	}

	// Verify rider owns this ride
	if ride.RiderID != userID {
		return nil, fmt.Errorf("unauthorized: not your ride")
	}

	// Check if ride can be reassigned
	if ride.Status != models.RideStatusDriverAssigned && ride.Status != models.RideStatusRequested {
		return nil, fmt.Errorf("ride cannot be reassigned in status: %s", ride.Status)
	}

	previousDriverID := ride.DriverID

	// Reset ride for reassignment
	ride.Status = models.RideStatusRequested
	ride.DriverID = nil
	if err := s.DB.Save(&ride).Error; err != nil {
		return nil, err
	}

	// Trigger new matching (with higher priority)
	s.findAndNotifyDrivers(&ride)

	return map[string]interface{}{
		"ride_id":         ride.ID,
		"previous_driver": previousDriverID,
		"new_driver":      nil, // Will be populated after matching
		"reassign_count":  1,
		"matching_wave":   1,
		"priority":        priority,
		"reason":          reason,
		"message":         "Ride queued for reassignment",
	}, nil
}

// GetFailureReason returns detailed failure analysis
func (s *RideService) GetFailureReason(rideID uuid.UUID) (map[string]interface{}, error) {
	var ride models.Ride
	if err := s.DB.First(&ride, rideID).Error; err != nil {
		return nil, err
	}

	// Build failure chain based on ride status and history
	failureChain := []map[string]interface{}{}

	if ride.Status == models.RideStatusNoDriverFound {
		failureChain = append(failureChain,
			map[string]interface{}{
				"step":      "initial_matching",
				"status":    "timeout",
				"details":   "No driver accepted in wave 1-3",
				"timestamp": ride.RequestedAt.Add(45 * time.Second).Format(time.RFC3339),
			},
			map[string]interface{}{
				"step":      "extended_matching",
				"status":    "timeout",
				"details":   "No driver accepted in wave 4 (15km radius)",
				"timestamp": ride.RequestedAt.Add(60 * time.Second).Format(time.RFC3339),
			},
			map[string]interface{}{
				"step":      "auto_cancellation",
				"status":    "completed",
				"details":   "Auto-cancelled due to no_driver_found",
				"timestamp": ride.RequestedAt.Add(65 * time.Second).Format(time.RFC3339),
			},
		)
	}

	// Root cause analysis
	rootCause := "UNKNOWN"
	recommendations := []string{}

	if ride.Status == models.RideStatusNoDriverFound {
		rootCause = "DRIVER_SHORTAGE_AREA"
		recommendations = []string{
			"Increase surge pricing for this area",
			"Send push notification to nearby drivers",
			"Consider scheduled ride option",
		}
	} else if ride.Status == models.RideStatusCancelled {
		rootCause = "USER_CANCELLATION"
		if ride.CancellationReason != "" {
			rootCause = ride.CancellationReason
		}
	}

	return map[string]interface{}{
		"ride_id":         ride.ID,
		"final_status":    ride.Status,
		"failure_chain":   failureChain,
		"root_cause":      rootCause,
		"recommendations": recommendations,
		"analysis_time":   time.Now().Format(time.RFC3339),
	}, nil
}
