package services

import (
	"context"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// ABTestingService manages experiments and variants
type ABTestingService struct {
	redis *redis.Client
}

// Experiment represents an A/B test
type Experiment struct {
	ID                string         `json:"id"`
	Name              string         `json:"name"`
	Description       string         `json:"description"`
	Variants          []Variant      `json:"variants"`
	TrafficAllocation map[string]int `json:"traffic_allocation"` // percentage per variant
	Status            string         `json:"status"`             // running, paused, completed
	StartDate         time.Time      `json:"start_date"`
	EndDate           *time.Time     `json:"end_date,omitempty"`
	TargetUsers       []string       `json:"target_users,omitempty"` // city_ids or user_segments
}

type Variant struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Config      map[string]interface{} `json:"config"`
}

// UserAssignment tracks which variant a user sees
type UserAssignment struct {
	UserID       string    `json:"user_id"`
	ExperimentID string    `json:"experiment_id"`
	VariantID    string    `json:"variant_id"`
	AssignedAt   time.Time `json:"assigned_at"`
}

func NewABTestingService(redis *redis.Client) *ABTestingService {
	return &ABTestingService{redis: redis}
}

// GetVariant assigns user to experiment variant (sticky assignment)
func (a *ABTestingService) GetVariant(experimentID, userID string) (string, map[string]interface{}, error) {
	// Check if user already assigned
	assignment := a.getUserAssignment(experimentID, userID)
	if assignment != nil {
		variant := a.getVariant(experimentID, assignment.VariantID)
		if variant != nil {
			return variant.ID, variant.Config, nil
		}
	}

	// Get experiment
	experiment := a.getExperiment(experimentID)
	if experiment == nil || experiment.Status != "running" {
		return "control", nil, nil // Return control if experiment not running
	}

	// Assign based on traffic allocation
	variantID := a.assignVariant(experiment, userID)
	variant := a.getVariant(experimentID, variantID)

	// Store assignment
	a.storeAssignment(experimentID, userID, variantID)

	// Record impression
	a.recordImpression(experimentID, variantID)

	if variant == nil {
		return variantID, nil, nil
	}
	return variant.ID, variant.Config, nil
}

// assignVariant uses weighted random selection
func (a *ABTestingService) assignVariant(experiment *Experiment, userID string) string {
	totalWeight := 0
	for _, weight := range experiment.TrafficAllocation {
		totalWeight += weight
	}

	if totalWeight == 0 {
		return "control"
	}

	// Use userID hash + current time for deterministic but random assignment
	seed := int64(hash(userID) + int(time.Now().Unix()/3600)) // Changes hourly
	r := rand.New(rand.NewSource(seed))
	randomValue := r.Intn(totalWeight)

	cumulative := 0
	for variantID, weight := range experiment.TrafficAllocation {
		cumulative += weight
		if randomValue < cumulative {
			return variantID
		}
	}
 
	return "control"
}

// RecordConversion records a conversion event for experiment analysis
func (a *ABTestingService) RecordConversion(experimentID, variantID, userID, eventType string, value float64) {
	key := "ab:conversion:" + experimentID + ":" + variantID + ":" + time.Now().Format("2006-01-02")
	a.redis.HIncrBy(context.Background(), key, eventType+"_count", 1)
	a.redis.HIncrByFloat(context.Background(), key, eventType+"_value", value)
}

// GetExperimentResults returns experiment metrics
func (a *ABTestingService) GetExperimentResults(experimentID string) map[string]interface{} {
	experiment := a.getExperiment(experimentID)
	if experiment == nil {
		return nil
	}

	results := map[string]interface{}{
		"experiment_id":   experimentID,
		"experiment_name": experiment.Name,
		"status":          experiment.Status,
		"variants":        []map[string]interface{}{},
	}

	for _, variant := range experiment.Variants {
		metrics := a.getVariantMetrics(experimentID, variant.ID)
		results["variants"] = append(results["variants"].([]map[string]interface{}), map[string]interface{}{
			"variant_id":      variant.ID,
			"variant_name":    variant.Name,
			"impressions":     metrics["impressions"],
			"conversions":     metrics["conversions"],
			"conversion_rate": metrics["conversion_rate"],
		})
	}

	return results
}

// CreateExperiment creates new experiment
func (a *ABTestingService) CreateExperiment(name, description string, variants []Variant, trafficAllocation map[string]int) (*Experiment, error) {
	experiment := &Experiment{
		ID:                uuid.New().String(),
		Name:              name,
		Description:       description,
		Variants:          variants,
		TrafficAllocation: trafficAllocation,
		Status:            "draft",
		StartDate:         time.Now(),
	}

	// Store experiment
	a.storeExperiment(experiment)

	return experiment, nil
}

// StartExperiment activates experiment
func (a *ABTestingService) StartExperiment(experimentID string) error {
	experiment := a.getExperiment(experimentID)
	if experiment == nil {
		return nil
	}
	experiment.Status = "running"
	a.storeExperiment(experiment)
	return nil
}

// StopExperiment stops experiment
func (a *ABTestingService) StopExperiment(experimentID string) error {
	experiment := a.getExperiment(experimentID)
	if experiment == nil {
		return nil
	}
	experiment.Status = "completed"
	now := time.Now()
	experiment.EndDate = &now
	a.storeExperiment(experiment)
	return nil
}

// Helper methods
func (a *ABTestingService) getUserAssignment(experimentID, userID string) *UserAssignment {
	key := "ab:assignment:" + experimentID + ":" + userID
	data, _ := a.redis.Get(context.Background(), key).Result()
	if data == "" {
		return nil
	}
	return &UserAssignment{
		UserID:       userID,
		ExperimentID: experimentID,
		VariantID:    data,
		AssignedAt:   time.Now(),
	}
}

func (a *ABTestingService) storeAssignment(experimentID, userID, variantID string) {
	key := "ab:assignment:" + experimentID + ":" + userID
	a.redis.Set(context.Background(), key, variantID, 30*24*time.Hour)
}

func (a *ABTestingService) getExperiment(experimentID string) *Experiment {
	// In production: fetch from DB/Redis
	return nil
}

func (a *ABTestingService) storeExperiment(experiment *Experiment) {
	// In production: store to DB/Redis
}

func (a *ABTestingService) getVariant(experimentID, variantID string) *Variant {
	experiment := a.getExperiment(experimentID)
	if experiment == nil {
		return nil
	}
	for _, v := range experiment.Variants {
		if v.ID == variantID {
			return &v
		}
	}
	return nil
}

func (a *ABTestingService) recordImpression(experimentID, variantID string) {
	key := "ab:impression:" + experimentID + ":" + variantID + ":" + time.Now().Format("2006-01-02")
	a.redis.Incr(context.Background(), key)
}

func (a *ABTestingService) getVariantMetrics(experimentID, variantID string) map[string]interface{} {
	date := time.Now().Format("2006-01-02")

	impressionKey := "ab:impression:" + experimentID + ":" + variantID + ":" + date
	conversionKey := "ab:conversion:" + experimentID + ":" + variantID + ":" + date

	impressions, _ := a.redis.Get(context.Background(), impressionKey).Int64()
	conversions, _ := a.redis.HGet(context.Background(), conversionKey, "conversion_count").Int64()

	conversionRate := 0.0
	if impressions > 0 {
		conversionRate = float64(conversions) / float64(impressions) * 100
	}

	return map[string]interface{}{
		"impressions":     impressions,
		"conversions":     conversions,
		"conversion_rate": conversionRate,
	}
}

func hash(s string) int {
	h := 0
	for _, c := range s {
		h = 31*h + int(c)
	}
	return h
}

// ABTestingEndpoints returns API endpoints
func GetABTestingEndpoints() []map[string]interface{} {
	return []map[string]interface{}{
		{"method": "GET", "path": "/experiments/variant", "desc": "Get variant for user", "params": "?experiment_id=xxx&user_id=yyy"},
		{"method": "POST", "path": "/experiments", "desc": "Create experiment"},
		{"method": "PUT", "path": "/experiments/:id/start", "desc": "Start experiment"},
		{"method": "PUT", "path": "/experiments/:id/stop", "desc": "Stop experiment"},
		{"method": "GET", "path": "/experiments/:id/results", "desc": "Get experiment results"},
		{"method": "POST", "path": "/experiments/:id/convert", "desc": "Record conversion event"},
	}
}

var ABTestingSvc *ABTestingService

func InitABTestingService(redis *redis.Client) {
	ABTestingSvc = NewABTestingService(redis)
}

func GetABTestingService() *ABTestingService {
	return ABTestingSvc
}
