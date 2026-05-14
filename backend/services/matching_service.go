package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"rapido-backend/database"
	"rapido-backend/models"
	"rapido-backend/utils"
	"rapido-backend/websocket"

	"github.com/google/uuid"
)

// MatchingService handles intelligent driver matching with dynamic radius
type MatchingService struct {
	searchStages []SearchStage
}

// SearchStage defines a search radius and time limit
type SearchStage struct {
	RadiusKM      float64
	TimeoutSec    int
	DispatchLimit int
	Label         string
}

// NewMatchingService creates a matching service with progressive expansion.
// Wave parameters are read from environment variables so they can be tuned
// without redeploying:
//
//	MATCHING_WAVE1_RADIUS_KM   (default 3)
//	MATCHING_WAVE1_TIMEOUT_SEC (default 5)
//	MATCHING_WAVE1_LIMIT       (default 3)
//	MATCHING_WAVE2_RADIUS_KM   (default 5)
//	MATCHING_WAVE2_TIMEOUT_SEC (default 5)
//	MATCHING_WAVE2_LIMIT       (default 5)
//	MATCHING_WAVE3_RADIUS_KM   (default 8)
//	MATCHING_WAVE3_TIMEOUT_SEC (default 10)
//	MATCHING_WAVE3_LIMIT       (default 10)
//	MATCHING_FALLBACK_RADIUS_KM   (default 12)
//	MATCHING_FALLBACK_TIMEOUT_SEC (default 15)
//	MATCHING_FALLBACK_LIMIT       (default 10)
func NewMatchingService() *MatchingService {
	envFloat := func(key string, def float64) float64 {
		if v := os.Getenv(key); v != "" {
			var f float64
			if _, err := fmt.Sscanf(v, "%f", &f); err == nil && f > 0 {
				return f
			}
		}
		return def
	}
	envInt := func(key string, def int) int {
		if v := os.Getenv(key); v != "" {
			var i int
			if _, err := fmt.Sscanf(v, "%d", &i); err == nil && i > 0 {
				return i
			}
		}
		return def
	}

	return &MatchingService{
		searchStages: []SearchStage{
			{RadiusKM: envFloat("MATCHING_WAVE1_RADIUS_KM", 3), TimeoutSec: envInt("MATCHING_WAVE1_TIMEOUT_SEC", 5), DispatchLimit: envInt("MATCHING_WAVE1_LIMIT", 3), Label: "wave_1"},
			{RadiusKM: envFloat("MATCHING_WAVE2_RADIUS_KM", 5), TimeoutSec: envInt("MATCHING_WAVE2_TIMEOUT_SEC", 5), DispatchLimit: envInt("MATCHING_WAVE2_LIMIT", 5), Label: "wave_2"},
			{RadiusKM: envFloat("MATCHING_WAVE3_RADIUS_KM", 8), TimeoutSec: envInt("MATCHING_WAVE3_TIMEOUT_SEC", 10), DispatchLimit: envInt("MATCHING_WAVE3_LIMIT", 10), Label: "wave_3"},
			{RadiusKM: envFloat("MATCHING_FALLBACK_RADIUS_KM", 12), TimeoutSec: envInt("MATCHING_FALLBACK_TIMEOUT_SEC", 15), DispatchLimit: envInt("MATCHING_FALLBACK_LIMIT", 10), Label: "fallback"},
		},
	}
}

// MatchResult contains the matching result
type MatchResult struct {
	Drivers       []MatchedDriver
	Stage         string
	RadiusUsed    float64
	HasMoreStages bool
}

type RideMatchingWaveState struct {
	RideID        string    `json:"ride_id"`
	Stage         string    `json:"stage"`
	RadiusKM      float64   `json:"radius_km"`
	DispatchLimit int       `json:"dispatch_limit"`
	DriverIDs     []string  `json:"driver_ids"`
	NotifiedAt    time.Time `json:"notified_at"`
	ExpiresAt     time.Time `json:"expires_at"`
}

// MatchedDriver represents a driver with match score
type MatchedDriver struct {
	DriverID       uuid.UUID
	DistanceKM     float64
	ETA            int
	Rating         float64
	MatchScore     float64
	AcceptanceRate float64
	VehicleType    string
	IdleMinutes    int // How long driver has been idle (new)
	RecentNotifies int // How many notifications in past hour (new)
}

// FindDriversForRide searches for drivers with progressive radius expansion
func (ms *MatchingService) FindDriversForRide(
	pickupLat, pickupLng float64,
	vehicleType string,
	femaleOnly bool,
	currentStage int,
) (*MatchResult, error) {

	if currentStage >= len(ms.searchStages) {
		return &MatchResult{
			Drivers:       []MatchedDriver{},
			Stage:         "no_drivers",
			RadiusUsed:    0,
			HasMoreStages: false,
		}, nil
	}

	stage := ms.searchStages[currentStage]
	ds := NewDriverService()

	// Search in current radius
	driverLocations, err := ds.GetNearbyDrivers(pickupLat, pickupLng, stage.RadiusKM, vehicleType, femaleOnly)
	if err != nil {
		return nil, err
	}

	// Score and sort drivers
	var matchedDrivers []MatchedDriver

	// Use the existing driver scoring service for multi-factor matching
	scoringService := NewDriverScoringService()
	scoredDrivers := scoringService.CalculateScores(driverLocations, pickupLat, pickupLng, vehicleType, models.RidePreferences{})

	for _, scoredDriver := range scoredDrivers {
		driverID, _ := uuid.Parse(scoredDriver.DriverID)
		matchedDrivers = append(matchedDrivers, MatchedDriver{
			DriverID:       driverID,
			DistanceKM:     scoredDriver.DistanceKM,
			ETA:            scoredDriver.ETA,
			Rating:         scoredDriver.Rating,
			MatchScore:     scoredDriver.MatchScore,
			AcceptanceRate: scoredDriver.AcceptanceRate,
			IdleMinutes:    int(scoredDriver.IdleTime.Minutes()),
		})
	}

	// Drivers already sorted by CalculateScores
	return &MatchResult{
		Drivers:       matchedDrivers,
		Stage:         stage.Label,
		RadiusUsed:    stage.RadiusKM,
		HasMoreStages: currentStage < len(ms.searchStages)-1,
	}, nil
}

// calculateMatchScore computes a weighted score for driver matching
func (ms *MatchingService) calculateMatchScore(distanceKM, rating, acceptanceRate float64) float64 {
	// Weights
	distanceWeight := 0.40   // Closer is better
	ratingWeight := 0.30     // Higher rating is better
	acceptanceWeight := 0.30 // Higher acceptance rate is better

	// Normalize distance (0-1, where 1 is best = 0km)
	distanceScore := 1 - (distanceKM / 15) // 15km max
	if distanceScore < 0 {
		distanceScore = 0
	}

	// Rating already 0-5, normalize to 0-1
	ratingScore := rating / 5

	// Acceptance rate already 0-1
	acceptanceScore := acceptanceRate

	// Calculate weighted score
	finalScore := (distanceScore * distanceWeight) +
		(ratingScore * ratingWeight) +
		(acceptanceScore * acceptanceWeight)

	return finalScore * 100 // Convert to 0-100 scale
}

// StartMatchingProcess initiates progressive matching waves with cleanup.
func (ms *MatchingService) StartMatchingProcess(ride *models.Ride) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		hub := websocket.GetHandler()
		requestStateKey := database.GetRideRequestStateKey(ride.ID.String())
		waveKey := database.GetRideWaveKey(ride.ID.String())
		pendingDriversKey := database.GetRidePendingDriversKey(ride.ID.String())
		notifiedDriverIDs := make(map[string]struct{})

		for pass := 0; pass < 2; pass++ {
			if pass == 1 {
				_ = hub.SendToUserEvent(ride.RiderID.String(), "ride_status_update", map[string]interface{}{
					"ride_id":  ride.ID.String(),
					"status":   models.RideStatusRequested,
					"retrying": true,
				})
			}

			for stageIndex, stage := range ms.searchStages {
				select {
				case <-ctx.Done():
					ms.handleNoDriverFound(ride.ID, ride.RiderID)
					return
				default:
				}

				if pass == 1 {
					stage.Label = stage.Label + "_retry"
					stage.DispatchLimit += 2
					stage.TimeoutSec += 2
				}

				var currentRide models.Ride
				if err := database.DB.First(&currentRide, ride.ID).Error; err != nil {
					log.Printf("matching stopped for ride %s: %v", ride.ID, err)
					return
				}
				if currentRide.Status != models.RideStatusRequested {
					ms.clearMatchingState(ride.ID.String(), ride.RiderID.String())
					return
				}

				result, err := ms.FindDriversForRide(ride.Pickup.Latitude, ride.Pickup.Longitude, ride.VehicleType, ride.Preferences.FemaleDriverOnly, stageIndex)
				if err != nil {
					log.Printf("matching error for ride %s: %v", ride.ID, err)
					continue
				}

				selected := ms.selectTopDrivers(result.Drivers, stage.DispatchLimit)
				selected = ms.filterUnnotifiedDrivers(selected, notifiedDriverIDs)
				if len(selected) == 0 {
					continue
				}

				driverIDs := make([]string, 0, len(selected))
				for _, driver := range selected {
					did := driver.DriverID.String()
					driverIDs = append(driverIDs, did)
					notifiedDriverIDs[did] = struct{}{}
				}

				state := RideMatchingWaveState{
					RideID:        ride.ID.String(),
					Stage:         stage.Label,
					RadiusKM:      stage.RadiusKM,
					DispatchLimit: stage.DispatchLimit,
					DriverIDs:     driverIDs,
					NotifiedAt:    time.Now(),
					ExpiresAt:     time.Now().Add(time.Duration(stage.TimeoutSec) * time.Second),
				}
				_ = database.SetCacheJSON(requestStateKey, state, 60*time.Second)
				_ = database.SetCacheJSON(waveKey, state, time.Duration(stage.TimeoutSec)*time.Second)
				_ = database.SetCacheJSON(pendingDriversKey, driverIDs, time.Duration(stage.TimeoutSec)*time.Second)

				ms.notifyDrivers(selected, ride.ID, stage.Label)
				_ = hub.SendToUserEvent(ride.RiderID.String(), "ride_status_update", map[string]interface{}{
					"ride_id":      ride.ID.String(),
					"status":       models.RideStatusRequested,
					"stage":        stage.Label,
					"driver_count": len(selected),
				})
				_ = hub.SendRideEvent(ride.ID.String(), "ride_request", map[string]interface{}{
					"ride_id":      ride.ID.String(),
					"stage":        stage.Label,
					"radius_km":    stage.RadiusKM,
					"driver_count": len(selected),
				})

				select {
				case <-ctx.Done():
					ms.handleNoDriverFound(ride.ID, ride.RiderID)
					return
				case <-time.After(time.Duration(stage.TimeoutSec) * time.Second):
				}

				if err := database.DB.First(&currentRide, ride.ID).Error; err != nil {
					return
				}
				if currentRide.Status != models.RideStatusRequested {
					ms.clearMatchingState(ride.ID.String(), ride.RiderID.String())
					return
				}
			}

			var currentRide models.Ride
			if err := database.DB.First(&currentRide, ride.ID).Error; err != nil {
				return
			}
			if currentRide.Status != models.RideStatusRequested {
				ms.clearMatchingState(ride.ID.String(), ride.RiderID.String())
				return
			}
		}

		ms.handleNoDriverFound(ride.ID, ride.RiderID)
	}()
}

// notifyDrivers sends push notifications to matched drivers

func (ms *MatchingService) notifyDrivers(drivers []MatchedDriver, rideID uuid.UUID, stage string) {
	hub := websocket.GetHandler()
	for _, driver := range drivers {
		_ = hub.SendToUserEvent(driver.DriverID.String(), "ride_request", map[string]interface{}{
			"ride_id":     rideID.String(),
			"stage":       stage,
			"driver_id":   driver.DriverID.String(),
			"distance_km": driver.DistanceKM,
			"eta_minutes": driver.ETA,
			"match_score": driver.MatchScore,
		})

		// Queue notification job via callback (avoids import cycle)
		if SubmitJobCallback != nil {
			SubmitJobCallback("send_notification", map[string]interface{}{
				"driver_id": driver.DriverID.String(),
				"ride_id":   rideID.String(),
				"title":     "New Ride Request",
				"body":      fmt.Sprintf("%.1f km away, ₹ estimated fare", driver.DistanceKM),
				"data": map[string]string{
					"type":    "ride_request",
					"ride_id": rideID.String(),
					"eta":     fmt.Sprintf("%d", driver.ETA),
				},
			})
		}
	}
}

func (ms *MatchingService) selectTopDrivers(drivers []MatchedDriver, limit int) []MatchedDriver {
	if limit <= 0 || len(drivers) <= limit {
		return drivers
	}
	return drivers[:limit]
}

func (ms *MatchingService) filterUnnotifiedDrivers(drivers []MatchedDriver, notified map[string]struct{}) []MatchedDriver {
	if len(notified) == 0 {
		return drivers
	}

	filtered := make([]MatchedDriver, 0, len(drivers))
	for _, d := range drivers {
		if _, exists := notified[d.DriverID.String()]; exists {
			continue
		}
		filtered = append(filtered, d)
	}
	return filtered
}

func (ms *MatchingService) handleNoDriverFound(rideID uuid.UUID, riderID uuid.UUID) {
	var currentRide models.Ride
	if err := database.DB.First(&currentRide, rideID).Error; err != nil {
		return
	}
	if currentRide.Status != models.RideStatusRequested {
		return
	}

	_ = database.DB.Model(&currentRide).Update("status", models.RideStatusNoDriverFound).Error
	_ = database.DeleteCache(database.GetRiderCurrentRideKey(riderID.String()))
	ms.clearMatchingState(rideID.String(), riderID.String())
	_ = websocket.GetHandler().SendToUserEvent(riderID.String(), "ride_status_update", map[string]interface{}{
		"ride_id": rideID.String(),
		"status":  models.RideStatusNoDriverFound,
	})
	_ = websocket.GetHandler().SendRideEvent(rideID.String(), "ride_status_update", map[string]interface{}{
		"ride_id": rideID.String(),
		"status":  models.RideStatusNoDriverFound,
	})
}

func (ms *MatchingService) clearMatchingState(rideID, riderID string) {
	_ = database.DeleteCache(database.GetRideRequestStateKey(rideID))
	_ = database.DeleteCache(database.GetRideWaveKey(rideID))
	_ = database.DeleteCache(database.GetRidePendingDriversKey(rideID))
	_ = database.DeleteCache(database.GetRideDriversNotifiedKey(rideID))
	_ = database.DeleteCache(database.GetRideStatusKey(rideID))
	if riderID != "" {
		_ = database.DeleteCache(database.GetRiderCurrentRideKey(riderID))
	}
}

// calculateDistance calculates distance between two points using Haversine formula
func calculateDistance(lat1, lng1, lat2, lng2 float64) float64 {
	// Import utils and use CalculateDistance
	return utils.CalculateDistance(lat1, lng1, lat2, lng2)
}

func mustMarshal(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}
