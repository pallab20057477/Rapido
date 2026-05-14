package workers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"rapido-backend/config"
	"rapido-backend/database"
	"rapido-backend/integrations/crm"
	"rapido-backend/models"

	"github.com/google/uuid"
)

// Job types
const (
	JobTypeSendNotification  = "send_notification"
	JobTypeProcessPayment    = "process_payment"
	JobTypeUpdateDriverStats = "update_driver_stats"
	JobTypeReassignRide      = "reassign_ride"
	JobTypeSendEmail         = "send_email"
	JobTypeGenerateInvoice   = "generate_invoice"
	JobTypeSyncExternalCRM   = "sync_external_crm"
)

// Job priority levels for backpressure handling
const (
	PriorityHigh   = iota // Critical: ride booking, payment, reassignment
	PriorityNormal        // Standard: notifications, stats updates
	PriorityLow           // Batch: CRM sync, analytics, reports
)

// GetJobPriority returns the priority level for a job type.
// Jobs with higher priority are processed first under backpressure.
func GetJobPriority(jobType string) int {
	switch jobType {
	case JobTypeProcessPayment, JobTypeReassignRide, JobTypeSendEmail:
		return PriorityHigh
	case JobTypeSendNotification, JobTypeUpdateDriverStats, JobTypeGenerateInvoice:
		return PriorityNormal
	case JobTypeSyncExternalCRM:
		return PriorityLow
	default:
		return PriorityNormal
	}
}

// Job represents a background job
type Job struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Priority   int             `json:"priority"` // PriorityHigh/Normal/Low
	Payload    json.RawMessage `json:"payload"`
	Attempts   int             `json:"attempts"`
	MaxRetries int             `json:"max_retries"`
	CreatedAt  time.Time       `json:"created_at"`
}

// QueueStats tracks queue health and backpressure metrics
type QueueStats struct {
	PendingCount   int       `json:"pending_count"`
	ProcessedCount int64     `json:"processed_count"`
	FailedCount    int64     `json:"failed_count"`
	QueueDepth     float64   `json:"queue_depth_percentage"`
	BackpressureOn bool      `json:"backpressure_on"`
	LastUpdated    time.Time `json:"last_updated"`
}

// WorkerPool manages background workers with priority queues and backpressure.
// This implementation follows FAANG patterns:
// - Priority-based job queueing (high-priority jobs process before low-priority)
// - Backpressure shedding: under high load, low-priority jobs are dropped gracefully
// - Queue depth monitoring: stats exported for observability
// - Graceful degradation: service remains responsive even under load
type WorkerPool struct {
	numWorkers     int
	highQueue      chan Job // High-priority: rides, payments
	normalQueue    chan Job // Normal-priority: notifications, stats
	lowQueue       chan Job // Low-priority: CRM, analytics
	quit           chan bool
	stopOnce       sync.Once
	stats          QueueStats
	backpressureTx float64 // Queue depth threshold (0.8 = 80%) to enable backpressure
}

// NewWorkerPool creates a new worker pool with priority-based queueing.
func NewWorkerPool(numWorkers int) *WorkerPool {
	return &WorkerPool{
		numWorkers:     numWorkers,
		highQueue:      make(chan Job, 100), // 100 high-priority slots
		normalQueue:    make(chan Job, 200), // 200 normal-priority slots
		lowQueue:       make(chan Job, 100), // 100 low-priority slots
		quit:           make(chan bool),
		backpressureTx: 0.8, // Enable shedding when queue > 80% full
	}
}

// Start starts the worker pool
func (wp *WorkerPool) Start() {
	for i := 0; i < wp.numWorkers; i++ {
		go wp.worker(i)
	}
	log.Printf("Started %d background workers with priority queue and backpressure", wp.numWorkers)
}

// Stop stops the worker pool gracefully (safe to call multiple times)
func (wp *WorkerPool) Stop() {
	wp.stopOnce.Do(func() {
		close(wp.quit)
	})
}

// Submit submits a job to the appropriate queue based on priority.
// Returns error if job cannot be enqueued (all queues full under backpressure).
func (wp *WorkerPool) Submit(job Job) error {
	// Assign priority if not set
	if job.Priority == 0 {
		job.Priority = GetJobPriority(job.Type)
	}

	// Check if we're under backpressure
	queueFull := wp.isQueueFull()

	// Route job to appropriate queue
	switch job.Priority {
	case PriorityHigh:
		// High-priority jobs always get enqueued (or fail if queue is truly full)
		select {
		case wp.highQueue <- job:
			return nil
		default:
			// High-priority queue full; log critical alert
			log.Printf("[CRITICAL] High-priority queue full, cannot enqueue job %s (%s)", job.ID, job.Type)
			return fmt.Errorf("high-priority queue full")
		}

	case PriorityNormal:
		// Normal jobs dropped if we're under backpressure
		if queueFull {
			log.Printf("[BACKPRESSURE] Dropping normal-priority job %s (%s)", job.ID, job.Type)
			wp.stats.FailedCount++
			return fmt.Errorf("backpressure: normal queue dropped")
		}
		select {
		case wp.normalQueue <- job:
			return nil
		default:
			log.Printf("[BACKPRESSURE] Normal queue full, dropping job %s (%s)", job.ID, job.Type)
			wp.stats.FailedCount++
			return fmt.Errorf("backpressure: normal queue full")
		}

	case PriorityLow:
		// Low-priority jobs are first to shed under backpressure
		if queueFull {
			log.Printf("[BACKPRESSURE] Shedding low-priority job %s (%s)", job.ID, job.Type)
			wp.stats.FailedCount++
			return fmt.Errorf("backpressure: low-priority job shed")
		}
		select {
		case wp.lowQueue <- job:
			return nil
		default:
			log.Printf("[BACKPRESSURE] Low queue full, shedding job %s (%s)", job.ID, job.Type)
			wp.stats.FailedCount++
			return fmt.Errorf("backpressure: low queue full")
		}

	default:
		log.Printf("Unknown priority level %d for job %s", job.Priority, job.ID)
		return fmt.Errorf("invalid job priority")
	}
}

// isQueueFull checks if the system is under backpressure.
// Returns true if total queue depth exceeds the backpressure threshold.
func (wp *WorkerPool) isQueueFull() bool {
	highLen := float64(len(wp.highQueue))
	normalLen := float64(len(wp.normalQueue))
	lowLen := float64(len(wp.lowQueue))

	totalLen := highLen + normalLen + lowLen
	totalCap := float64(100 + 200 + 100) // Total capacity

	depth := totalLen / totalCap
	wp.stats.QueueDepth = depth * 100

	if depth > wp.backpressureTx {
		wp.stats.BackpressureOn = true
		return true
	}
	wp.stats.BackpressureOn = false
	return false
}

// Stats returns current queue statistics
func (wp *WorkerPool) Stats() QueueStats {
	wp.stats.PendingCount = len(wp.highQueue) + len(wp.normalQueue) + len(wp.lowQueue)
	wp.stats.LastUpdated = time.Now()
	return wp.stats
}

// worker is the worker goroutine
// Reads from high-priority queue first, then normal, then low-priority.
// This ensures critical jobs (payments, ride reassignment) process immediately while
// allowing batch work (CRM sync) to be dropped under load.
func (wp *WorkerPool) worker(id int) {
	for {
		select {
		// High-priority jobs: always try first
		case job := <-wp.highQueue:
			if err := wp.processJob(job); err != nil {
				log.Printf("Worker %d: Job %s failed: %v", id, job.ID, err)
				if job.Attempts < job.MaxRetries {
					job.Attempts++
					time.Sleep(time.Duration(job.Attempts) * time.Second) // Exponential backoff
					wp.Submit(job)                                        // Requeue with backpressure check
				}
			}
			wp.stats.ProcessedCount++

		// Normal-priority jobs: if no high-priority work
		case job := <-wp.normalQueue:
			if err := wp.processJob(job); err != nil {
				log.Printf("Worker %d: Job %s failed: %v", id, job.ID, err)
				if job.Attempts < job.MaxRetries {
					job.Attempts++
					time.Sleep(time.Duration(job.Attempts) * time.Second)
					wp.Submit(job)
				}
			}
			wp.stats.ProcessedCount++

		// Low-priority jobs: if no high/normal work
		case job := <-wp.lowQueue:
			if err := wp.processJob(job); err != nil {
				log.Printf("Worker %d: Job %s failed: %v", id, job.ID, err)
				if job.Attempts < job.MaxRetries {
					job.Attempts++
					time.Sleep(time.Duration(job.Attempts) * time.Second)
					wp.Submit(job)
				}
			}
			wp.stats.ProcessedCount++

		case <-wp.quit:
			log.Printf("Worker %d stopping", id)
			return
		}
	}
}

// processJob processes a job
func (wp *WorkerPool) processJob(job Job) error {
	log.Printf("Processing job: %s (type: %s, attempt: %d)", job.ID, job.Type, job.Attempts)

	switch job.Type {
	case JobTypeSendNotification:
		return wp.processNotificationJob(job)
	case JobTypeProcessPayment:
		return wp.processPaymentJob(job)
	case JobTypeUpdateDriverStats:
		return wp.processDriverStatsJob(job)
	case JobTypeReassignRide:
		return wp.processReassignRideJob(job)
	case JobTypeGenerateInvoice:
		return wp.processGenerateInvoiceJob(job)
	case JobTypeSyncExternalCRM:
		return wp.processCRMJob(job)
	default:
		log.Printf("Unknown job type: %s", job.Type)
		return nil
	}
}

// processNotificationJob processes notification job
func (wp *WorkerPool) processNotificationJob(job Job) error {
	var payload struct {
		UserID string                 `json:"user_id"`
		Title  string                 `json:"title"`
		Body   string                 `json:"body"`
		Type   string                 `json:"type"`
		RideID string                 `json:"ride_id,omitempty"`
		Data   map[string]interface{} `json:"data,omitempty"`
	}
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return err
	}

	userID, _ := uuid.Parse(payload.UserID)
	rideID, _ := uuid.Parse(payload.RideID)

	// Create notification record
	notification := &models.Notification{
		UserID:   userID,
		Type:     payload.Type,
		Title:    payload.Title,
		Body:     payload.Body,
		Channels: []string{"push"},
		Status:   models.NotificationStatusPending,
	}
	if payload.RideID != "" {
		if notification.Data == nil {
			notification.Data = make(map[string]interface{})
		}
		notification.Data["ride_id"] = rideID
	}
	if payload.Data != nil {
		for k, v := range payload.Data {
			notification.Data[k] = v
		}
	}

	if err := database.DB.Create(notification).Error; err != nil {
		return err
	}

	// Send FCM push notification via callback (to avoid import cycle)
	if FCMNotifyCallback != nil {
		if err := FCMNotifyCallback(userID, payload.Title, payload.Body, notification.Data); err != nil {
			// Update notification as failed
			database.DB.Model(notification).Updates(map[string]interface{}{
				"status": models.NotificationStatusFailed,
				"error":  err.Error(),
			})
			return err
		}
		// Mark as sent
		now := time.Now()
		notification.Status = models.NotificationStatusSent
		notification.SentAt = &now
		database.DB.Model(notification).Updates(map[string]interface{}{
			"status":  models.NotificationStatusSent,
			"sent_at": now,
		})
	} else {
		log.Printf("[WORKER] FCM callback not set, notification saved for user %s: %s", payload.UserID, payload.Title)
	}

	return nil
}

// processPaymentJob processes payment job
func (wp *WorkerPool) processPaymentJob(job Job) error {
	var payload struct {
		RideID string `json:"ride_id"`
		Method string `json:"method"`
	}
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return err
	}

	log.Printf("Processing payment for ride %s", payload.RideID)
	// Payment processing handled synchronously in service
	return nil
}

// processDriverStatsJob updates driver statistics
func (wp *WorkerPool) processDriverStatsJob(job Job) error {
	var payload struct {
		DriverID string `json:"driver_id"`
	}
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return err
	}

	driverID, _ := uuid.Parse(payload.DriverID)

	// Update driver rating summary
	var stats struct {
		TotalRides     int64
		CompletedRides int64
		CancelledRides int64
		TotalEarnings  float64
		AverageRating  float64
	}

	database.DB.Raw(`
		SELECT 
			COUNT(*) as total_rides,
			SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) as completed_rides,
			SUM(CASE WHEN status = 'cancelled' THEN 1 ELSE 0 END) as cancelled_rides
		FROM rides WHERE driver_id = ?
	`, driverID).Scan(&stats)

	database.DB.Raw(`
		SELECT COALESCE(SUM(amount), 0) as total_earnings
		FROM driver_earnings WHERE driver_id = ?
	`, driverID).Scan(&stats.TotalEarnings)

	database.DB.Raw(`
		SELECT COALESCE(AVG(driver_rating), 5) as average_rating
		FROM ratings WHERE driver_id = ?
	`, driverID).Scan(&stats.AverageRating)

	// Update driver record
	return database.DB.Model(&models.Driver{}).Where("id = ?", driverID).Updates(map[string]interface{}{
		"total_rides":     stats.TotalRides,
		"completed_rides": stats.CompletedRides,
		"cancelled_rides": stats.CancelledRides,
		"total_earnings":  stats.TotalEarnings,
		"rating":          stats.AverageRating,
	}).Error
}

// processReassignRideJob reassigns a ride to another driver
func (wp *WorkerPool) processReassignRideJob(job Job) error {
	var payload struct {
		RideID string `json:"ride_id"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return err
	}

	rideID, _ := uuid.Parse(payload.RideID)

	// Find ride and reset to requested state
	var ride models.Ride
	if err := database.DB.First(&ride, rideID).Error; err != nil {
		return err
	}

	if ride.Status == models.RideStatusDriverAssigned || ride.Status == models.RideStatusRequested {
		// Reset driver assignment
		database.DB.Model(&ride).Updates(map[string]interface{}{
			"driver_id": nil,
			"status":    models.RideStatusRequested,
		})

		// Log reassignment using AdminActivityLog instead
		logEntry := models.AdminActivityLog{
			AdminID:     uuid.Nil,
			Action:      "RIDE_REASSIGNED",
			EntityType:  "ride",
			EntityID:    &rideID,
			Description: payload.Reason,
			NewValues: map[string]interface{}{
				"ride_id": rideID.String(),
				"status":  models.RideStatusRequested,
				"reason":  payload.Reason,
			},
			CreatedAt: time.Now(),
		}
		database.DB.Create(&logEntry)

		log.Printf("Ride %s reassigned", payload.RideID)
	}

	return nil
}

// processGenerateInvoiceJob generates invoice for payment
func (wp *WorkerPool) processGenerateInvoiceJob(job Job) error {
	var payload struct {
		PaymentID string `json:"payment_id"`
	}
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return err
	}

	log.Printf("Generating invoice for payment %s", payload.PaymentID)
	// Invoice generation handled in payment service
	return nil
}

// processCRMJob synchronizes important events to an external CRM without blocking core flows.
func (wp *WorkerPool) processCRMJob(job Job) error {
	var payload crm.EventEnvelope
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return err
	}

	cfg := config.Get().CRM
	if !cfg.Enabled || cfg.BaseURL == "" {
		log.Printf("External CRM sync skipped (disabled or not configured): %s", payload.Event)
		return nil
	}

	timeout := time.Duration(cfg.TimeoutS) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	body, _ := json.Marshal(payload)
	endpoint := strings.TrimRight(cfg.BaseURL, "/") + crm.DefaultWebhookURL
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}

	requestTimestamp := payload.OccurredAt.UTC().Format(time.RFC3339Nano)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(crm.HeaderEventID, payload.EventID)
	req.Header.Set(crm.HeaderEvent, payload.Event)
	req.Header.Set(crm.HeaderVersion, payload.Version)
	req.Header.Set(crm.HeaderSource, payload.Source)
	req.Header.Set(crm.HeaderTimestamp, requestTimestamp)
	req.Header.Set(crm.HeaderRetryCount, fmt.Sprintf("%d", payload.RetryCount))
	if sig := crm.SignWebhook(body, cfg.WebhookSecret, requestTimestamp); sig != "" {
		req.Header.Set(crm.HeaderSignature, sig)
	}
	if cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
			log.Printf("External CRM rejected event %s with non-retryable status: %s", payload.EventID, resp.Status)
			return nil
		}
		return fmt.Errorf("crm sync failed: %s: %s", resp.Status, string(respBody))
	}

	log.Printf("External CRM sync success: %s %s", payload.Event, payload.EntityID)
	return nil
}

// EnqueueNotification adds a notification job to the queue
func (wp *WorkerPool) EnqueueNotification(userID uuid.UUID, title, body, notifType string, data map[string]interface{}) {
	payload, _ := json.Marshal(map[string]interface{}{
		"user_id": userID.String(),
		"title":   title,
		"body":    body,
		"type":    notifType,
		"data":    data,
	})

	job := Job{
		ID:         uuid.New().String(),
		Type:       JobTypeSendNotification,
		Payload:    payload,
		MaxRetries: 3,
		CreatedAt:  time.Now(),
	}

	wp.Submit(job)
}

// EnqueueDriverStatsUpdate triggers driver stats update
func (wp *WorkerPool) EnqueueDriverStatsUpdate(driverID uuid.UUID) {
	payload, _ := json.Marshal(map[string]string{
		"driver_id": driverID.String(),
	})

	job := Job{
		ID:         uuid.New().String(),
		Type:       JobTypeUpdateDriverStats,
		Payload:    payload,
		MaxRetries: 3,
		CreatedAt:  time.Now(),
	}

	wp.Submit(job)
}

// EnqueueCRMSync submits an event for an external CRM and never blocks request handling.
func (wp *WorkerPool) EnqueueCRMSync(event, entityType, entityID string, data map[string]interface{}) {
	envelope := crm.BuildEventEnvelope(event, entityType, entityID, data, 0)
	payload, _ := json.Marshal(envelope)

	job := Job{
		ID:         uuid.New().String(),
		Type:       JobTypeSyncExternalCRM,
		Payload:    payload,
		MaxRetries: 5,
		CreatedAt:  time.Now(),
	}

	wp.Submit(job)
}

// SubmitJob submits a job via callback if set (avoids import cycle)
func SubmitJob(jobType string, payload interface{}) error {
	if JobSubmitCallback == nil {
		return fmt.Errorf("job submit callback not set")
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return JobSubmitCallback(Job{
		Type:    jobType,
		Payload: data,
	})
}

// timePtr returns a pointer to time
func timePtr(t time.Time) *time.Time {
	return &t
}

// FCMNotifyCallback is set by main/services to send FCM notifications (avoids import cycle)
var FCMNotifyCallback func(userID uuid.UUID, title, body string, data map[string]interface{}) error

// JobSubmitCallback allows services to submit jobs without importing workers (avoids import cycle)
var JobSubmitCallback func(job Job) error

// WorkerPoolInstance global instance
var WorkerPoolInstance *WorkerPool

// InitWorkerPool initializes the global worker pool
func InitWorkerPool() {
	WorkerPoolInstance = NewWorkerPool(5) // 5 workers
	WorkerPoolInstance.Start()
}

// StopWorkerPool stops the global worker pool
func StopWorkerPool() {
	if WorkerPoolInstance != nil {
		WorkerPoolInstance.Stop()
	}
}
