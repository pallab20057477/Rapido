package services

import (
	"fmt"
	"math"
	"time"

	"rapido-backend/database"
	"rapido-backend/models"
	"rapido-backend/utils"
)

// DriverScoringService provides intelligent driver ranking
type DriverScoringService struct{}

// ScoredDriver represents a driver with computed match score
type ScoredDriver struct {
	DriverID         string
	DistanceKM       float64
	ETA              int
	Rating           float64
	AcceptanceRate   float64
	TotalRides       int
	IdleTime         time.Duration
	CancellationRate float64
	MatchScore       float64
	ScoreBreakdown   ScoreComponents
}

// ScoreComponents shows how score was calculated
type ScoreComponents struct {
	DistanceScore   float64
	RatingScore     float64
	AcceptanceScore float64
	IdleScore       float64
	PenaltyScore    float64
}

// NewDriverScoringService creates a new scoring service
func NewDriverScoringService() *DriverScoringService {
	return &DriverScoringService{}
}

// CalculateScores computes match scores for all candidate drivers
func (s *DriverScoringService) CalculateScores(
	candidates []models.DriverLocation,
	pickupLat, pickupLng float64,
	vehicleType string,
	preferences models.RidePreferences,
) []ScoredDriver {

	var scored []ScoredDriver

	for _, loc := range candidates {
		// Calculate distance
		distance := utils.CalculateDistance(pickupLat, pickupLng, loc.Latitude, loc.Longitude)

		// Get driver stats
		driver, stats := s.getDriverStats(loc.DriverID.String())

		// Calculate ETA
		eta := utils.CalculateETA(distance, vehicleType)

		// Calculate idle time
		idleTime := time.Since(loc.UpdatedAt)

		// Calculate score components
		score, breakdown := s.calculateScore(
			distance,
			driver.Rating,
			stats.AcceptanceRate,
			stats.CancellationRate,
			idleTime,
			stats.TotalRides,
			preferences,
		)

		scored = append(scored, ScoredDriver{
			DriverID:         loc.DriverID.String(),
			DistanceKM:       distance,
			ETA:              eta,
			Rating:           driver.Rating,
			AcceptanceRate:   stats.AcceptanceRate,
			TotalRides:       stats.TotalRides,
			IdleTime:         idleTime,
			CancellationRate: stats.CancellationRate,
			MatchScore:       score,
			ScoreBreakdown:   breakdown,
		})
	}

	// Sort by score (higher is better)
	return s.sortByScore(scored)
}

// calculateScore computes weighted score with penalties
func (s *DriverScoringService) calculateScore(
	distanceKM float64,
	rating float64,
	acceptanceRate float64,
	cancellationRate float64,
	idleTime time.Duration,
	totalRides int,
	preferences models.RidePreferences,
) (float64, ScoreComponents) {

	// Weights (sum = 1.0)
	const (
		distanceWeight   = 0.30
		ratingWeight     = 0.25
		acceptanceWeight = 0.20
		idleWeight       = 0.15
		experienceWeight = 0.10
	)

	// 1. Distance Score (0-100, closer = better)
	// Exponential decay: score = 100 * e^(-distance/3)
	distanceScore := 100 * math.Exp(-distanceKM/3.0)

	// 2. Rating Score (0-100, higher rating = better)
	// Normalize 0-5 to 0-100
	ratingScore := (rating / 5.0) * 100

	// 3. Acceptance Rate Score (0-100)
	acceptanceScore := acceptanceRate * 100

	// 4. Idle Time Score (0-100, longer idle = higher priority)
	// Max score at 10 minutes, then plateaus
	idleMinutes := idleTime.Minutes()
	idleScore := math.Min(idleMinutes/10.0, 1.0) * 100

	// 5. Experience Score (0-100, more rides = slightly better)
	// Logarithmic scale: log10(rides+1) * 20, max at 100
	experienceScore := math.Min(math.Log10(float64(totalRides+1))*20, 100)

	// Calculate weighted score
	weightedScore :=
		distanceScore*distanceWeight +
			ratingScore*ratingWeight +
			acceptanceScore*acceptanceWeight +
			idleScore*idleWeight +
			experienceScore*experienceWeight

	// Apply penalties
	penalty := s.calculatePenalty(cancellationRate, preferences)
	finalScore := weightedScore - penalty

	// Ensure score is within 0-100
	finalScore = math.Max(0, math.Min(100, finalScore))

	return finalScore, ScoreComponents{
		DistanceScore:   distanceScore,
		RatingScore:     ratingScore,
		AcceptanceScore: acceptanceScore,
		IdleScore:       idleScore,
		PenaltyScore:    -penalty,
	}
}

// calculatePenalty computes penalty for negative factors
func (s *DriverScoringService) calculatePenalty(cancellationRate float64, preferences models.RidePreferences) float64 {
	penalty := 0.0

	// High cancellation rate penalty
	if cancellationRate > 0.10 { // More than 10% cancellation
		penalty += (cancellationRate - 0.10) * 200 // Up to -20 points
	}

	return penalty
}

// getDriverStats retrieves driver performance stats
func (s *DriverScoringService) getDriverStats(driverID string) (models.Driver, DriverStats) {
	var driver models.Driver
	database.DB.First(&driver, driverID)

	var stats DriverStats

	// Get acceptance rate from last 100 rides
	database.DB.Raw(`
		SELECT 
			COUNT(*) as total_rides,
			COALESCE(SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END), 0) as completed_rides,
			COALESCE(SUM(CASE WHEN status = 'cancelled' THEN 1 ELSE 0 END), 0) as cancelled_rides,
			COALESCE(AVG(driver_rating), 0) as avg_rating
		FROM rides 
		WHERE driver_id = ? 
		AND created_at > NOW() - INTERVAL '30 days'
	`, driverID).Scan(&stats)

	// Calculate rates
	if stats.TotalRides > 0 {
		stats.AcceptanceRate = float64(stats.CompletedRides) / float64(stats.TotalRides)
		stats.CancellationRate = float64(stats.CancelledRides) / float64(stats.TotalRides)
	} else {
		stats.AcceptanceRate = 1.0 // New drivers get benefit of doubt
		stats.CancellationRate = 0
	}

	return driver, stats
}

// sortByScore sorts drivers by match score (descending)
func (s *DriverScoringService) sortByScore(drivers []ScoredDriver) []ScoredDriver {
	// Bubble sort (for small lists, simple is fine)
	n := len(drivers)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if drivers[j].MatchScore < drivers[j+1].MatchScore {
				drivers[j], drivers[j+1] = drivers[j+1], drivers[j]
			}
		}
	}
	return drivers
}

// SelectTopDrivers returns top N drivers with score threshold
func (s *DriverScoringService) SelectTopDrivers(
	scored []ScoredDriver,
	count int,
	minScore float64,
) []ScoredDriver {
	var selected []ScoredDriver

	for _, driver := range scored {
		if driver.MatchScore >= minScore {
			selected = append(selected, driver)
			if len(selected) >= count {
				break
			}
		}
	}

	return selected
}

// GetScoreExplanation returns human-readable score explanation
func (s *DriverScoringService) GetScoreExplanation(driver ScoredDriver) string {
	return fmt.Sprintf(
		"Driver %s: Score %.1f (Distance: %.1f, Rating: %.1f%%, Acceptance: %.1f%%, Idle: %.0fm)",
		driver.DriverID[:8],
		driver.MatchScore,
		driver.ScoreBreakdown.DistanceScore,
		driver.ScoreBreakdown.RatingScore,
		driver.ScoreBreakdown.AcceptanceScore,
		driver.IdleTime.Minutes(),
	)
}

// DriverStats holds aggregated driver performance
type DriverStats struct {
	TotalRides       int
	CompletedRides   int
	CancelledRides   int
	AcceptanceRate   float64
	CancellationRate float64
	AverageRating    float64
}

// RejectPenalty applies penalty when driver rejects ride
func (s *DriverScoringService) RejectPenalty(driverID string) {
	// Track rejection for future scoring
	key := "driver:rejections:" + driverID
	database.RedisClient.Incr(database.Ctx, key)
	database.RedisClient.Expire(database.Ctx, key, 24*time.Hour)
}

// AcceptBonus applies bonus when driver accepts ride
func (s *DriverScoringService) AcceptBonus(driverID string) {
	// Clear recent rejections
	key := "driver:rejections:" + driverID
	database.RedisClient.Del(database.Ctx, key)
}
