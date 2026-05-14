package services

import (
	"context"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// IncentiveEngine manages driver incentives, bonuses, gamification
type IncentiveEngine struct {
	redis *redis.Client
}

type Incentive struct {
	ID           string                 `json:"id"`
	DriverID     string                 `json:"driver_id"`
	Type         string                 `json:"type"` // bonus, streak, quest, referral
	Amount       float64                `json:"amount"`
	Currency     string                 `json:"currency"`
	Status       string                 `json:"status"` // pending, completed, claimed, expired
	StartTime    time.Time              `json:"start_time"`
	EndTime      time.Time              `json:"end_time"`
	Requirements map[string]interface{} `json:"requirements"`
	Progress     map[string]interface{} `json:"progress"`
}

type DailyTarget struct {
	DriverID       string  `json:"driver_id"`
	Date           string  `json:"date"`
	TargetRides    int     `json:"target_rides"`
	CompletedRides int     `json:"completed_rides"`
	TargetAmount   float64 `json:"target_amount"`
	EarnedAmount   float64 `json:"earned_amount"`
	BonusAmount    float64 `json:"bonus_amount"`
	Status         string  `json:"status"` // active, completed, missed
}

type StreakReward struct {
	DriverID      string    `json:"driver_id"`
	CurrentStreak int       `json:"current_streak"`
	MaxStreak     int       `json:"max_streak"`
	RewardAmount  float64   `json:"reward_amount"`
	LastRideDate  time.Time `json:"last_ride_date"`
}

func NewIncentiveEngine(redis *redis.Client) *IncentiveEngine {
	return &IncentiveEngine{redis: redis}
}

// CalculateDailyTargets sets targets based on driver's history
func (i *IncentiveEngine) CalculateDailyTargets(driverID string) *DailyTarget {
	// Get driver's average daily rides (last 30 days)
	avgRides := i.getDriverAverageRides(driverID, 30)

	// Set target 20% above average
	targetRides := int(math.Ceil(avgRides * 1.2))
	if targetRides < 5 {
		targetRides = 5 // Minimum target
	}

	return &DailyTarget{
		DriverID:       driverID,
		Date:           time.Now().Format("2006-01-02"),
		TargetRides:    targetRides,
		CompletedRides: 0,
		TargetAmount:   float64(targetRides) * 150, // ₹150 avg per ride
		EarnedAmount:   0,
		BonusAmount:    0,
		Status:         "active",
	}
}

// UpdateProgress updates driver progress toward targets
func (i *IncentiveEngine) UpdateProgress(driverID string, rideCompleted bool, amount float64) {
	today := time.Now().Format("2006-01-02")
	key := "target:" + driverID + ":" + today

	// Increment completed rides
	if rideCompleted {
		i.redis.HIncrBy(context.Background(), key, "completed_rides", 1)
		i.redis.HIncrByFloat(context.Background(), key, "earned_amount", amount)
	}

	// Check if targets reached
	target := i.GetDailyTarget(driverID)
	if target != nil && target.CompletedRides >= target.TargetRides {
		// Award completion bonus
		bonus := target.TargetAmount * 0.1 // 10% bonus
		i.redis.HSet(context.Background(), key, "bonus_amount", bonus)
		i.redis.HSet(context.Background(), key, "status", "completed")

		// Notify driver
		i.notifyDriver(driverID, "target_completed", map[string]interface{}{
			"bonus":   bonus,
			"message": "Congratulations! You completed your daily target!",
		})
	}
}

// CalculateStreakReward calculates consecutive days bonus
func (i *IncentiveEngine) CalculateStreakReward(driverID string) *StreakReward {
	streak := i.getCurrentStreak(driverID)

	// Streak rewards
	var rewardAmount float64
	switch {
	case streak >= 30:
		rewardAmount = 2000 // ₹2000 for 30-day streak
	case streak >= 14:
		rewardAmount = 800 // ₹800 for 14-day streak
	case streak >= 7:
		rewardAmount = 300 // ₹300 for 7-day streak
	case streak >= 3:
		rewardAmount = 100 // ₹100 for 3-day streak
	default:
		rewardAmount = 0
	}

	return &StreakReward{
		DriverID:      driverID,
		CurrentStreak: streak,
		RewardAmount:  rewardAmount,
		LastRideDate:  time.Now(),
	}
}

// CreateQuest creates a quest-style challenge
func (i *IncentiveEngine) CreateQuest(questType string, requirements map[string]interface{}) *Incentive {
	quest := &Incentive{
		ID:           uuid.New().String(),
		Type:         "quest",
		Status:       "pending",
		StartTime:    time.Now(),
		EndTime:      time.Now().Add(7 * 24 * time.Hour), // 7 days
		Requirements: requirements,
		Progress:     make(map[string]interface{}),
	}

	// Set amount based on quest difficulty
	switch questType {
	case "weekend_warrior":
		quest.Amount = 500
		quest.Requirements = map[string]interface{}{
			"rides_needed": 20,
			"time_window":  "weekend",
		}
	case "peak_hour_hero":
		quest.Amount = 800
		quest.Requirements = map[string]interface{}{
			"peak_rides":  15,
			"time_window": "peak_hours",
		}
	case "long_distance_champion":
		quest.Amount = 1000
		quest.Requirements = map[string]interface{}{
			"long_rides":   10, // >15km
			"min_distance": 15.0,
		}
	case "acceptance_master":
		quest.Amount = 300
		quest.Requirements = map[string]interface{}{
			"acceptance_rate": 0.95,
			"min_rides":       30,
		}
	}

	return quest
}

// GetDailyTarget retrieves driver's daily target
func (i *IncentiveEngine) GetDailyTarget(driverID string) *DailyTarget {
	today := time.Now().Format("2006-01-02")
	key := "target:" + driverID + ":" + today

	data, err := i.redis.HGetAll(context.Background(), key).Result()
	if err != nil || len(data) == 0 {
		return i.CalculateDailyTargets(driverID)
	}

	return &DailyTarget{
		DriverID:       driverID,
		Date:           today,
		TargetRides:    parseInt(data["target_rides"]),
		CompletedRides: parseInt(data["completed_rides"]),
		TargetAmount:   parseFloat(data["target_amount"]),
		EarnedAmount:   parseFloat(data["earned_amount"]),
		BonusAmount:    parseFloat(data["bonus_amount"]),
		Status:         data["status"],
	}
}

// Helper methods
func (i *IncentiveEngine) getDriverAverageRides(driverID string, days int) float64 {
	// Query from analytics DB
	return 8.5 // Default average
}

func (i *IncentiveEngine) getCurrentStreak(driverID string) int {
	key := "streak:" + driverID
	streak, _ := i.redis.Get(context.Background(), key).Int()
	return streak
}

func (i *IncentiveEngine) notifyDriver(driverID string, event string, data map[string]interface{}) {
	// Send push notification
}

// GetIncentiveDashboard returns driver's incentive dashboard
func (i *IncentiveEngine) GetIncentiveDashboard(driverID string) map[string]interface{} {
	target := i.GetDailyTarget(driverID)
	streak := i.CalculateStreakReward(driverID)

	progress := 0.0
	if target.TargetRides > 0 {
		progress = float64(target.CompletedRides) / float64(target.TargetRides) * 100
	}

	return map[string]interface{}{
		"daily_target": map[string]interface{}{
			"target_rides":    target.TargetRides,
			"completed_rides": target.CompletedRides,
			"progress":        progress,
			"target_amount":   target.TargetAmount,
			"earned_amount":   target.EarnedAmount,
			"potential_bonus": target.BonusAmount,
			"status":          target.Status,
		},
		"streak": map[string]interface{}{
			"current":        streak.CurrentStreak,
			"reward":         streak.RewardAmount,
			"next_milestone": i.getNextMilestone(streak.CurrentStreak),
		},
		"available_quests": []map[string]interface{}{
			{"name": "Weekend Warrior", "reward": 500, "progress": "0/20 rides"},
			{"name": "Peak Hour Hero", "reward": 800, "progress": "0/15 rides"},
			{"name": "Long Distance Champ", "reward": 1000, "progress": "0/10 rides"},
		},
	}
}

func (i *IncentiveEngine) getNextMilestone(current int) map[string]interface{} {
	milestones := []int{3, 7, 14, 30}
	rewards := []float64{100, 300, 800, 2000}

	for idx, m := range milestones {
		if current < m {
			return map[string]interface{}{
				"rides_needed": m - current,
				"reward":       rewards[idx],
			}
		}
	}
	return map[string]interface{}{"rides_needed": 0, "reward": 0}
}

// IncentiveEngineEndpoints returns API endpoints
func GetIncentiveEngineEndpoints() []map[string]interface{} {
	return []map[string]interface{}{
		{"method": "GET", "path": "/drivers/incentives/dashboard", "desc": "Get incentive dashboard"},
		{"method": "GET", "path": "/drivers/incentives/targets", "desc": "Get daily targets"},
		{"method": "GET", "path": "/drivers/incentives/streak", "desc": "Get streak info"},
		{"method": "GET", "path": "/drivers/incentives/quests", "desc": "Get available quests"},
		{"method": "POST", "path": "/drivers/incentives/quests/:id/claim", "desc": "Claim quest reward"},
	}
}

var IncentiveEng *IncentiveEngine

func InitIncentiveEngine(redis *redis.Client) {
	IncentiveEng = NewIncentiveEngine(redis)
}

func GetIncentiveEngine() *IncentiveEngine {
	return IncentiveEng
}
