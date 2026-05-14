package services

import (
	"errors"
	"time"

	"rapido-backend/database"
	"rapido-backend/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type DriverService struct {
	DB *gorm.DB
}

func NewDriverService() *DriverService {
	return &DriverService{DB: database.DB}
}

// RegisterDriver creates a new driver profile
func (s *DriverService) RegisterDriver(userID uuid.UUID, req RegisterDriverRequest) (*models.Driver, error) {
	// Verify user exists in database
	var user models.User
	if err := s.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found - please login again")
		}
		return nil, err
	}

	// Check if driver already exists
	var existing models.Driver
	if err := s.DB.Where("user_id = ?", userID).First(&existing).Error; err == nil {
		return nil, errors.New("driver profile already exists")
	}

	// Update user role to driver
	if err := s.DB.Model(&user).Update("role", "driver").Error; err != nil {
		return nil, err
	}

	// Create driver
	driver := &models.Driver{
		UserID:        userID,
		LicenseNumber: req.LicenseNumber,
		LicenseImage:  req.LicenseImage,
		RCNumber:      req.RCNumber,
		RCImage:       req.RCImage,
		AadhaarNumber: req.AadhaarNumber,
		AadhaarImage:  req.AadhaarImage,
		IsVerified:    false,
		IsOnline:      false,
		IsActive:      true,
		Languages:     req.Languages,
	}

	if req.LicenseExpiry != nil {
		driver.LicenseExpiry = req.LicenseExpiry
	}

	if err := s.DB.Create(driver).Error; err != nil {
		return nil, err
	}

	// Create vehicle
	vehicle := &models.Vehicle{
		DriverID:     driver.ID,
		Type:         req.VehicleType,
		Make:         req.VehicleMake,
		Model:        req.VehicleModel,
		Year:         req.VehicleYear,
		Color:        req.VehicleColor,
		NumberPlate:  req.VehicleNumberPlate,
		FuelType:     req.FuelType,
		VehicleImage: req.VehicleImage,
	}

	if err := s.DB.Create(vehicle).Error; err != nil {
		return nil, err
	}

	// Create driver earnings record
	earnings := &models.DriverEarnings{
		DriverID: driver.ID,
	}
	if err := s.DB.Create(earnings).Error; err != nil {
		return nil, err
	}

	// Create driver rating summary
	ratingSummary := &models.DriverRatingSummary{
		DriverID: driver.ID,
	}
	if err := s.DB.Create(ratingSummary).Error; err != nil {
		return nil, err
	}

	return driver, nil
}

type RegisterDriverRequest struct {
	LicenseNumber      string     `json:"license_number"`
	LicenseImage       string     `json:"license_image"`
	LicenseExpiry      *time.Time `json:"license_expiry,omitempty"`
	RCNumber           string     `json:"rc_number"`
	RCImage            string     `json:"rc_image"`
	AadhaarNumber      string     `json:"aadhaar_number"`
	AadhaarImage       string     `json:"aadhaar_image"`
	VehicleType        string     `json:"vehicle_type"`
	VehicleMake        string     `json:"vehicle_make"`
	VehicleModel       string     `json:"vehicle_model"`
	VehicleYear        int        `json:"vehicle_year"`
	VehicleColor       string     `json:"vehicle_color"`
	VehicleNumberPlate string     `json:"vehicle_number_plate"`
	FuelType           string     `json:"fuel_type"`
	VehicleImage       string     `json:"vehicle_image"`
	Languages          []string   `json:"languages"`
}

// GetDriverProfile gets driver profile
func (s *DriverService) GetDriverProfile(userID uuid.UUID) (*models.Driver, error) {
	var driver models.Driver
	if err := s.DB.Preload("User").Preload("Vehicle").Where("user_id = ?", userID).First(&driver).Error; err != nil {
		return nil, err
	}
	return &driver, nil
}

// UpdateDriverProfile updates driver profile
func (s *DriverService) UpdateDriverProfile(userID uuid.UUID, updates map[string]interface{}) (*models.Driver, error) {
	// Find driver by user ID
	var driver models.Driver
	if err := s.DB.Where("user_id = ?", userID).First(&driver).Error; err != nil {
		return nil, err
	}

	if err := s.DB.Model(&driver).Updates(updates).Error; err != nil {
		return nil, err
	}

	return &driver, nil
}

// GoOnline marks driver as online
func (s *DriverService) GoOnline(userID uuid.UUID, lat, lng float64) error {
	now := time.Now()

	// Check if driver is verified
	var driver models.Driver
	if err := s.DB.Where("user_id = ?", userID).First(&driver).Error; err != nil {
		return err
	}
	if !driver.IsVerified {
		return errors.New("driver not verified - pending admin approval")
	}

	// Update driver status
	if err := s.DB.Model(&models.Driver{}).Where("id = ?", driver.ID).Updates(map[string]interface{}{
		"is_online":  true,
		"updated_at": now,
	}).Error; err != nil {
		return err
	}

	// Update or create driver location
	var location models.DriverLocation
	err := s.DB.Where("driver_id = ?", driver.ID).First(&location).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			location = models.DriverLocation{
				DriverID:  driver.ID,
				Latitude:  lat,
				Longitude: lng,
				UpdatedAt: now,
			}
			if err := s.DB.Create(&location).Error; err != nil {
				return err
			}
		} else {
			return err
		}
	} else {
		location.Latitude = lat
		location.Longitude = lng
		location.UpdatedAt = now
		if err := s.DB.Save(&location).Error; err != nil {
			return err
		}
	}

	// Add to Redis geo index
	database.UpdateDriverGeoLocation(driver.ID.String(), lat, lng)

	// Set driver online status in Redis
	database.SetCache(database.GetDriverOnlineKey(driver.ID.String()), "1", time.Hour)
	database.SetCache(database.GetAvailableDriversKey()+":"+driver.ID.String(), "1", time.Hour)

	// Log status change
	log := &models.DriverStatusLog{
		DriverID:  driver.ID,
		IsOnline:  true,
		Latitude:  lat,
		Longitude: lng,
	}
	s.DB.Create(log)

	return nil
}

// GoOffline marks driver as offline
func (s *DriverService) GoOffline(userID uuid.UUID) error {
	// Find driver by user ID
	var driver models.Driver
	if err := s.DB.Where("user_id = ?", userID).First(&driver).Error; err != nil {
		return err
	}

	// Check if driver has active ride
	currentRideKey := database.GetDriverCurrentRideKey(driver.ID.String())
	if rideID, err := database.GetCache(currentRideKey); err == nil && rideID != "" {
		return errors.New("cannot go offline with active ride: " + rideID)
	}

	now := time.Now()

	if err := s.DB.Model(&models.Driver{}).Where("id = ?", driver.ID).Updates(map[string]interface{}{
		"is_online":  false,
		"updated_at": now,
	}).Error; err != nil {
		return err
	}

	// Remove from Redis geo index
	database.RemoveDriverGeoLocation(driver.ID.String())

	// Remove online status
	database.DeleteCache(database.GetDriverOnlineKey(driver.ID.String()))
	database.DeleteCache(database.GetAvailableDriversKey() + ":" + driver.ID.String())

	// Log status change
	log := &models.DriverStatusLog{
		DriverID: driver.ID,
		IsOnline: false,
	}
	s.DB.Create(log)

	return nil
}

// UpdateLocation updates driver location
func (s *DriverService) UpdateLocation(userID uuid.UUID, lat, lng float64, accuracy float64) error {
	now := time.Now()

	// Find driver by user ID
	var driver models.Driver
	if err := s.DB.Where("user_id = ?", userID).First(&driver).Error; err != nil {
		return err
	}

	// Update database; if no row exists, create it
	res := s.DB.Model(&models.DriverLocation{}).Where("driver_id = ?", driver.ID).Updates(map[string]interface{}{
		"latitude":   lat,
		"longitude":  lng,
		"accuracy":   accuracy,
		"updated_at": now,
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		// create new location row
		location := models.DriverLocation{
			DriverID:  driver.ID,
			Latitude:  lat,
			Longitude: lng,
			Accuracy:  accuracy,
			UpdatedAt: now,
		}
		if err := s.DB.Create(&location).Error; err != nil {
			return err
		}
	}

	// Update Redis if online
	onlineKey := database.GetDriverOnlineKey(driver.ID.String())
	if _, err := database.GetCache(onlineKey); err == nil {
		database.UpdateDriverGeoLocation(driver.ID.String(), lat, lng)
	}

	return nil
}

// GetDriverEarnings gets driver earnings
func (s *DriverService) GetDriverEarnings(userID uuid.UUID) (*models.DriverEarnings, error) {
	// Find driver by user ID
	var driver models.Driver
	if err := s.DB.Where("user_id = ?", userID).First(&driver).Error; err != nil {
		return nil, err
	}

	var earnings models.DriverEarnings
	if err := s.DB.Where("driver_id = ?", driver.ID).First(&earnings).Error; err != nil {
		return nil, err
	}
	return &earnings, nil
}

// GetNearbyDrivers finds drivers near a location
func (s *DriverService) GetNearbyDrivers(lat, lng, radiusKM float64, vehicleType string, femaleOnly bool) ([]models.DriverLocation, error) {
	// Find drivers in Redis geo radius
	results, err := database.FindNearbyDrivers(lat, lng, radiusKM)
	if err != nil {
		return nil, err
	}

	var locations []models.DriverLocation
	for _, result := range results {
		driverID, err := uuid.Parse(result.Name)
		if err != nil {
			continue
		}

		var driver models.Driver
		if err := s.DB.Where("id = ? AND is_online = ? AND is_verified = ? AND is_active = ?",
			driverID, true, true, true).First(&driver).Error; err != nil {
			continue
		}

		// Check female driver preference
		if femaleOnly && !driver.IsFemale {
			continue
		}

		// Check vehicle type if specified
		if vehicleType != "" {
			var vehicle models.Vehicle
			if err := s.DB.Where("driver_id = ? AND type = ? AND is_active = ?",
				driverID, vehicleType, true).First(&vehicle).Error; err != nil {
				continue
			}
		}

		// Check if driver has current ride
		currentRideKey := database.GetDriverCurrentRideKey(driverID.String())
		if rideID, err := database.GetCache(currentRideKey); err == nil && rideID != "" {
			continue
		}

		locations = append(locations, models.DriverLocation{
			DriverID:  driverID,
			Latitude:  result.Latitude,
			Longitude: result.Longitude,
		})
	}

	return locations, nil
}

// GetDriverStats gets driver statistics
func (s *DriverService) GetDriverStats(userID uuid.UUID) (map[string]interface{}, error) {
	// Find driver by user ID
	var driver models.Driver
	if err := s.DB.Where("user_id = ?", userID).First(&driver).Error; err != nil {
		return nil, err
	}

	var earnings models.DriverEarnings
	if err := s.DB.Where("driver_id = ?", driver.ID).First(&earnings).Error; err != nil {
		return nil, err
	}

	var rating models.DriverRatingSummary
	if err := s.DB.Where("driver_id = ?", driver.ID).First(&rating).Error; err != nil {
		return nil, err
	}

	// Get today's rides count
	today := time.Now().Truncate(24 * time.Hour)
	var todayRides int64
	s.DB.Model(&models.Ride{}).Where("driver_id = ? AND DATE(created_at) = DATE(?)", driver.ID, today).Count(&todayRides)

	// Calculate acceptance score from ride request logs
	acceptanceScore := s.calculateAcceptanceScore(driver.ID)

	return map[string]interface{}{
		"total_earnings":   earnings.TotalEarnings,
		"current_balance":  earnings.CurrentBalance,
		"daily_earnings":   earnings.DailyEarnings,
		"weekly_earnings":  earnings.WeeklyEarnings,
		"total_rides":      earnings.TotalRides,
		"today_rides":      todayRides,
		"rating":           rating.AverageRating,
		"total_ratings":    rating.TotalRatings,
		"acceptance_score": acceptanceScore,
	}, nil
}

// calculateAcceptanceScore calculates driver acceptance score based on ride request history
// Formula: (accepted requests / total requests) * 100, with penalties for timeouts
func (s *DriverService) calculateAcceptanceScore(driverID uuid.UUID) float64 {
	// Get last 50 ride requests for this driver (rolling window)
	var requestLogs []models.RideRequestLog
	s.DB.Where("driver_id = ?", driverID).
		Order("created_at DESC").
		Limit(50).
		Find(&requestLogs)

	if len(requestLogs) == 0 {
		// No history, return default score
		return 100.0
	}

	var accepted, rejected, timeout int
	for _, log := range requestLogs {
		switch log.Status {
		case "accepted":
			accepted++
		case "rejected":
			rejected++
		case "timeout":
			timeout++
		}
	}

	total := accepted + rejected + timeout
	if total == 0 {
		return 100.0
	}

	// Base score: acceptance rate
	baseScore := float64(accepted) / float64(total) * 100

	// Penalties
	timeoutPenalty := float64(timeout) * 2.0 // -2 points per timeout
	rejectPenalty := float64(rejected) * 0.5 // -0.5 points per reject

	score := baseScore - timeoutPenalty - rejectPenalty

	// Clamp between 0 and 100
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return score
}

// VerifyDriver (Admin only) - verifies a driver
func (s *DriverService) VerifyDriver(driverID, adminID uuid.UUID) error {
	now := time.Now()
	return s.DB.Model(&models.Driver{}).Where("id = ?", driverID).Updates(map[string]interface{}{
		"is_verified": true,
		"verified_at": now,
		"verified_by": adminID,
		"updated_at":  now,
	}).Error
}

// RejectDriver (Admin only) - rejects a driver
func (s *DriverService) RejectDriver(driverID uuid.UUID, reason string) error {
	return s.DB.Model(&models.Driver{}).Where("id = ?", driverID).Updates(map[string]interface{}{
		"is_verified": false,
		"is_active":   false,
		"updated_at":  time.Now(),
	}).Error
}

// GetPendingVerifications gets drivers pending verification
func (s *DriverService) GetPendingVerifications(page, perPage int) ([]models.Driver, int64, error) {
	var drivers []models.Driver
	var count int64

	offset := (page - 1) * perPage

	s.DB.Model(&models.Driver{}).Where("is_verified = ?", false).Count(&count)

	if err := s.DB.Preload("User").Preload("Vehicle").
		Where("is_verified = ?", false).
		Offset(offset).Limit(perPage).
		Find(&drivers).Error; err != nil {
		return nil, 0, err
	}

	return drivers, count, nil
}

// UpdateEarnings updates driver earnings after ride
func (s *DriverService) UpdateEarnings(driverID uuid.UUID, amount float64) error {
	return s.DB.Model(&models.DriverEarnings{}).Where("driver_id = ?", driverID).Updates(map[string]interface{}{
		"total_earnings":   gorm.Expr("total_earnings + ?", amount),
		"current_balance":  gorm.Expr("current_balance + ?", amount),
		"daily_earnings":   gorm.Expr("daily_earnings + ?", amount),
		"weekly_earnings":  gorm.Expr("weekly_earnings + ?", amount),
		"monthly_earnings": gorm.Expr("monthly_earnings + ?", amount),
		"last_updated":     time.Now(),
	}).Error
}

// ResetDailyEarnings resets daily earnings (called by cron job)
func (s *DriverService) ResetDailyEarnings() error {
	return s.DB.Model(&models.DriverEarnings{}).Update("daily_earnings", 0).Error
}

// ResetWeeklyEarnings resets weekly earnings
func (s *DriverService) ResetWeeklyEarnings() error {
	return s.DB.Model(&models.DriverEarnings{}).Update("weekly_earnings", 0).Error
}

// ResetMonthlyEarnings resets monthly earnings
func (s *DriverService) ResetMonthlyEarnings() error {
	return s.DB.Model(&models.DriverEarnings{}).Update("monthly_earnings", 0).Error
}
