package services

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/redis/go-redis/v9"
)

// BusinessMetrics tracks KPIs for business intelligence
type BusinessMetrics struct {
	redis *redis.Client
}

func NewBusinessMetrics(redis *redis.Client) *BusinessMetrics {
	return &BusinessMetrics{redis: redis}
}

// RecordRideSuccess records completed ride
func (b *BusinessMetrics) RecordRideSuccess(rideID, driverID string, duration float64) {
	now := time.Now()
	dateKey := now.Format("2006-01-02")

	// Increment daily success count
	b.redis.HIncrBy(context.Background(), "metrics:rides:"+dateKey, "completed", 1)
	b.redis.HIncrBy(context.Background(), "metrics:drivers:"+dateKey+":"+driverID, "completed", 1)

	// Add duration for average calculation
	b.redis.HIncrByFloat(context.Background(), "metrics:rides:"+dateKey, "total_duration", duration)
}

// RecordRideFailure records cancelled/failed ride
func (b *BusinessMetrics) RecordRideFailure(rideID, reason string, stage string) {
	now := time.Now()
	dateKey := now.Format("2006-01-02")

	b.redis.HIncrBy(context.Background(), "metrics:rides:"+dateKey, "cancelled", 1)
	b.redis.HIncrBy(context.Background(), "metrics:rides:"+dateKey, "cancel_reason:"+reason, 1)
	b.redis.HIncrBy(context.Background(), "metrics:rides:"+dateKey, "cancel_stage:"+stage, 1)
}

// RecordDriverAccept records driver acceptance
func (b *BusinessMetrics) RecordDriverAccept(driverID string, accepted bool) {
	now := time.Now()
	dateKey := now.Format("2006-01-02")

	if accepted {
		b.redis.HIncrBy(context.Background(), "metrics:drivers:"+dateKey+":"+driverID, "accepted", 1)
	} else {
		b.redis.HIncrBy(context.Background(), "metrics:drivers:"+dateKey+":"+driverID, "rejected", 1)
	}
}

// RecordETA records estimated vs actual time
func (b *BusinessMetrics) RecordETA(estimated, actual float64) {
	now := time.Now()
	dateKey := now.Format("2006-01-02")

	diff := actual - estimated
	b.redis.HIncrByFloat(context.Background(), "metrics:eta:"+dateKey, "total_diff", diff)
	b.redis.HIncrBy(context.Background(), "metrics:eta:"+dateKey, "count", 1)
}

// GetDailyKPIs returns daily business metrics
func (b *BusinessMetrics) GetDailyKPIs(date string) map[string]interface{} {
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	// Get all metrics for date
	rideMetrics, _ := b.redis.HGetAll(context.Background(), "metrics:rides:"+date).Result()

	completed := parseInt(rideMetrics["completed"])
	cancelled := parseInt(rideMetrics["cancelled"])
	total := completed + cancelled

	successRate := 0.0
	if total > 0 {
		successRate = float64(completed) / float64(total) * 100
	}

	avgDuration := 0.0
	if completed > 0 {
		totalDuration := parseFloat(rideMetrics["total_duration"])
		avgDuration = totalDuration / float64(completed)
	}

	return map[string]interface{}{
		"date":              date,
		"rides_completed":   completed,
		"rides_cancelled":   cancelled,
		"total_rides":       total,
		"success_rate":      round(successRate, 2),
		"cancellation_rate": round(100-successRate, 2),
		"avg_ride_duration": round(avgDuration, 1),
	}
}

// GetDriverAcceptanceRate returns driver acceptance metrics
func (b *BusinessMetrics) GetDriverAcceptanceRate(driverID string, days int) map[string]interface{} {
	accepted := 0
	rejected := 0

	for i := 0; i < days; i++ {
		date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		metrics, _ := b.redis.HGetAll(context.Background(), "metrics:drivers:"+date+":"+driverID).Result()
		accepted += parseInt(metrics["accepted"])
		rejected += parseInt(metrics["rejected"])
	}

	total := accepted + rejected
	acceptanceRate := 0.0
	if total > 0 {
		acceptanceRate = float64(accepted) / float64(total) * 100
	}

	return map[string]interface{}{
		"driver_id":       driverID,
		"period_days":     days,
		"offers_accepted": accepted,
		"offers_rejected": rejected,
		"total_offers":    total,
		"acceptance_rate": round(acceptanceRate, 2),
	}
}

// GetETAAccuracy returns ETA accuracy metrics
func (b *BusinessMetrics) GetETAAccuracy(days int) map[string]interface{} {
	totalDiff := 0.0
	count := 0

	for i := 0; i < days; i++ {
		date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		metrics, _ := b.redis.HGetAll(context.Background(), "metrics:eta:"+date).Result()
		totalDiff += parseFloat(metrics["total_diff"])
		count += parseInt(metrics["count"])
	}

	avgDiff := 0.0
	if count > 0 {
		avgDiff = totalDiff / float64(count)
	}

	accuracy := "excellent"
	if avgDiff > 5 {
		accuracy = "good"
	}
	if avgDiff > 10 {
		accuracy = "fair"
	}
	if avgDiff > 15 {
		accuracy = "poor"
	}

	return map[string]interface{}{
		"period_days":     days,
		"avg_eta_diff":    round(avgDiff, 1),
		"accuracy_rating": accuracy,
		"total_samples":   count,
	}
}

// GetDashboardMetrics returns executive dashboard data
func (b *BusinessMetrics) GetDashboardMetrics() map[string]interface{} {
	today := b.GetDailyKPIs("")
	yesterday := b.GetDailyKPIs(time.Now().AddDate(0, 0, -1).Format("2006-01-02"))

	// Calculate trends
	completedTrend := 0.0
	if yesterday["rides_completed"].(int) > 0 {
		completedTrend = float64(today["rides_completed"].(int)-yesterday["rides_completed"].(int)) / float64(yesterday["rides_completed"].(int)) * 100
	}

	return map[string]interface{}{
		"today":     today,
		"yesterday": yesterday,
		"trends": map[string]interface{}{
			"completed_rides": round(completedTrend, 1),
		},
		"top_kpis": []map[string]string{
			{"name": "Ride Success Rate", "value": today["success_rate"].(string) + "%", "target": "95%"},
			{"name": "Cancellation Rate", "value": today["cancellation_rate"].(string) + "%", "target": "<5%"},
			{"name": "Avg Ride Duration", "value": today["avg_ride_duration"].(string) + " min", "target": "25 min"},
		},
	}
}

// Helper functions
func parseInt(s string) int {
	if s == "" {
		return 0
	}
	var i int
	fmt.Sscanf(s, "%d", &i)
	return i
}

func parseFloat(s string) float64 {
	if s == "" {
		return 0
	}
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

func round(val float64, precision int) float64 {
	p := math.Pow(10, float64(precision))
	return math.Round(val*p) / p
}

var BusinessMetricsSvc *BusinessMetrics

func InitBusinessMetrics(redis *redis.Client) {
	BusinessMetricsSvc = NewBusinessMetrics(redis)
}

func GetBusinessMetrics() *BusinessMetrics {
	return BusinessMetricsSvc
}
