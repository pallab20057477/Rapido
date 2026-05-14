package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"rapido-backend/database"
	"rapido-backend/utils"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Job represents a queued job
type Job struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	Payload     json.RawMessage `json:"payload"`
	Priority    int             `json:"priority"` // 0=high, 1=normal, 2=low
	Attempts    int             `json:"attempts"`
	MaxRetries  int             `json:"max_retries"`
	CreatedAt   time.Time       `json:"created_at"`
	ScheduledAt time.Time       `json:"scheduled_at"` // For delayed jobs
}

// RedisQueue implements a reliable queue using Redis
type RedisQueue struct {
	client  *redis.Client
	ctx     context.Context
	queues  map[string]string // queue name -> redis key
	workers int
}

// NewRedisQueue creates a new Redis-backed queue
func NewRedisQueue(workers int) *RedisQueue {
	return &RedisQueue{
		client:  database.RedisClient,
		ctx:     context.Background(),
		queues:  make(map[string]string),
		workers: workers,
	}
}

// RegisterQueue registers a named queue
func (q *RedisQueue) RegisterQueue(name string) {
	q.queues[name] = "queue:" + name
}

// Enqueue adds a job to the queue
func (q *RedisQueue) Enqueue(queueName string, job *Job) error {
	queueKey, exists := q.queues[queueName]
	if !exists {
		return fmt.Errorf("queue %s not registered", queueName)
	}

	if job.ID == "" {
		job.ID = uuid.New().String()
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = time.Now()
	}
	if job.ScheduledAt.IsZero() {
		job.ScheduledAt = time.Now()
	}

	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	// Use Redis sorted set for priority + scheduled jobs
	score := float64(job.ScheduledAt.Unix())
	if job.Priority > 0 {
		// Lower priority = higher score delay
		score += float64(job.Priority * 60) // 60 sec per priority level
	}

	return q.client.ZAdd(q.ctx, queueKey, redis.Z{
		Score:  score,
		Member: string(data),
	}).Err()
}

// EnqueueImmediate adds a high-priority job immediately
func (q *RedisQueue) EnqueueImmediate(queueName string, jobType string, payload interface{}) error {
	data, _ := json.Marshal(payload)
	job := &Job{
		Type:       jobType,
		Payload:    data,
		Priority:   0, // High priority
		MaxRetries: 3,
	}
	return q.Enqueue(queueName, job)
}

// Dequeue retrieves and removes a job from the queue
func (q *RedisQueue) Dequeue(queueName string, timeout time.Duration) (*Job, error) {
	queueKey, exists := q.queues[queueName]
	if !exists {
		return nil, fmt.Errorf("queue %s not registered", queueName)
	}

	// Use Redis Lua script for atomic pop
	script := `
		local jobs = redis.call('zrangebyscore', KEYS[1], '-inf', ARGV[1], 'limit', 0, 1)
		if #jobs > 0 then
			redis.call('zrem', KEYS[1], jobs[1])
			return jobs[1]
		end
		return nil
	`

	now := float64(time.Now().Unix())
	result, err := q.client.Eval(q.ctx, script, []string{queueKey}, now).Result()
	if err != nil {
		return nil, err
	}

	if result == nil {
		// No job available, block with timeout using BLPOP pattern
		// For priority queues, we poll with short sleep
		return nil, nil
	}

	var job Job
	if err := json.Unmarshal([]byte(result.(string)), &job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job: %w", err)
	}

	return &job, nil
}

// MarkSuccess marks job as completed (move to success list)
func (q *RedisQueue) MarkSuccess(queueName string, job *Job) error {
	successKey := q.queues[queueName] + ":success"
	data, _ := json.Marshal(job)
	return q.client.LPush(q.ctx, successKey, data).Err()
}

// MarkFailed marks job as failed (move to dead letter queue)
func (q *RedisQueue) MarkFailed(queueName string, job *Job, err error) error {
	failedKey := q.queues[queueName] + ":failed"

	failedJob := struct {
		*Job
		Error    string    `json:"error"`
		FailedAt time.Time `json:"failed_at"`
	}{
		Job:      job,
		Error:    err.Error(),
		FailedAt: time.Now(),
	}

	data, _ := json.Marshal(failedJob)
	return q.client.LPush(q.ctx, failedKey, data).Err()
}

// RetryJob re-queues a failed job with backoff
func (q *RedisQueue) RetryJob(queueName string, job *Job) error {
	if job.Attempts >= job.MaxRetries {
		return fmt.Errorf("max retries exceeded")
	}

	job.Attempts++
	// Exponential backoff: 1s, 2s, 4s, 8s...
	backoff := time.Duration(1<<uint(job.Attempts-1)) * time.Second
	job.ScheduledAt = time.Now().Add(backoff)

	return q.Enqueue(queueName, job)
}

// GetQueueStats returns queue statistics
func (q *RedisQueue) GetQueueStats(queueName string) (map[string]int64, error) {
	queueKey, exists := q.queues[queueName]
	if !exists {
		return nil, fmt.Errorf("queue %s not registered", queueName)
	}

	pending, _ := q.client.ZCard(q.ctx, queueKey).Result()
	success, _ := q.client.LLen(q.ctx, queueKey+":success").Result()
	failed, _ := q.client.LLen(q.ctx, queueKey+":failed").Result()

	return map[string]int64{
		"pending": pending,
		"success": success,
		"failed":  failed,
	}, nil
}

// StartWorkers starts the worker pool
func (q *RedisQueue) StartWorkers(handler map[string]func(*Job) error) {
	for i := 0; i < q.workers; i++ {
		go q.worker(i, handler)
	}
	utils.Info("Redis queue workers started", zap.Int("count", q.workers))
}

// worker is the worker goroutine
func (q *RedisQueue) worker(id int, handlers map[string]func(*Job) error) {
	for {
		for queueName := range q.queues {
			job, err := q.Dequeue(queueName, 1*time.Second)
			if err != nil {
				utils.Error("Queue dequeue error", zap.Error(err), zap.String("queue", queueName))
				continue
			}
			if job == nil {
				continue
			}

			handler, exists := handlers[job.Type]
			if !exists {
				utils.Error("No handler for job type", zap.String("type", job.Type))
				_ = q.MarkFailed(queueName, job, fmt.Errorf("no handler"))
				continue
			}

			// Process job
			if err := handler(job); err != nil {
				utils.Error("Job failed", zap.Error(err), zap.String("job_id", job.ID), zap.Int("attempt", job.Attempts))

				if job.Attempts < job.MaxRetries {
					_ = q.RetryJob(queueName, job)
				} else {
					_ = q.MarkFailed(queueName, job, err)
				}
			} else {
				_ = q.MarkSuccess(queueName, job)
			}
		}
		time.Sleep(100 * time.Millisecond) // Prevent CPU spinning
	}
}

// GracefulShutdown stops workers gracefully
func (q *RedisQueue) GracefulShutdown(timeout time.Duration) {
	utils.Info("Queue graceful shutdown initiated", zap.Duration("timeout", timeout))
	// In production, implement proper shutdown with context cancellation
	time.Sleep(timeout)
}

// Global instance
var Queue *RedisQueue

// InitQueue initializes the global queue
func InitQueue(workers int) {
	Queue = NewRedisQueue(workers)

	// Register standard queues
	Queue.RegisterQueue("notifications")
	Queue.RegisterQueue("payments")
	Queue.RegisterQueue("sms")
	Queue.RegisterQueue("driver_stats")
	Queue.RegisterQueue("crm_sync")
	Queue.RegisterQueue("fraud_check")
}
