package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// KafkaQueueService manages async message processing
type KafkaQueueService struct {
	queues map[string]chan QueueMessage
}

type QueueMessage struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Payload    map[string]interface{} `json:"payload"`
	Timestamp  int64                  `json:"timestamp"`
	RetryCount int                    `json:"retry_count"`
}

// Queue types for async processing
const (
	QueueRideMatching   = "ride_matching"
	QueueNotifications  = "notifications"
	QueuePayments       = "payments"
	QueueEmailSMS       = "email_sms"
	QueueDriverLocation = "driver_location"
	QueueAnalytics      = "analytics"
)

func NewKafkaQueueService() *KafkaQueueService {
	service := &KafkaQueueService{
		queues: make(map[string]chan QueueMessage),
	}

	// Initialize all queues
	queues := []string{
		QueueRideMatching,
		QueueNotifications,
		QueuePayments,
		QueueEmailSMS,
		QueueDriverLocation,
		QueueAnalytics,
	}

	for _, q := range queues {
		service.queues[q] = make(chan QueueMessage, 1000)
	}

	return service
}

// Publish sends message to queue
func (s *KafkaQueueService) Publish(queueType string, payload map[string]interface{}) (string, error) {
	queue, exists := s.queues[queueType]
	if !exists {
		return "", fmt.Errorf("unknown queue: %s", queueType)
	}

	msg := QueueMessage{
		ID:         uuid.New().String(),
		Type:       queueType,
		Payload:    payload,
		Timestamp:  time.Now().Unix(),
		RetryCount: 0,
	}

	// Async publish
	select {
	case queue <- msg:
		return msg.ID, nil
	default:
		return "", fmt.Errorf("queue %s is full", queueType)
	}
}

// Consume processes messages (would be called by workers)
func (s *KafkaQueueService) Consume(queueType string, handler func(QueueMessage) error) {
	queue := s.queues[queueType]

	go func() {
		for msg := range queue {
			if err := handler(msg); err != nil {
				// Retry logic
				if msg.RetryCount < 3 {
					msg.RetryCount++
					time.Sleep(time.Duration(msg.RetryCount) * time.Second)
					queue <- msg
				}
			}
		}
	}()
}

// RideCreatedFlow - Main workflow: Ride Created → Kafka → Matching → Driver Notify
func (s *KafkaQueueService) RideCreatedFlow(rideID string, riderID string, pickup, dropoff map[string]float64) error {
	// Step 1: Queue ride matching
	_, err := s.Publish(QueueRideMatching, map[string]interface{}{
		"ride_id":   rideID,
		"rider_id":  riderID,
		"pickup":    pickup,
		"dropoff":   dropoff,
		"event":     "ride_created",
		"timestamp": time.Now().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}

	// Step 2: Queue notifications
	_, err = s.Publish(QueueNotifications, map[string]interface{}{
		"ride_id":      rideID,
		"type":         "ride_searching",
		"recipient_id": riderID,
		"message":      "Finding nearby drivers...",
	})

	return err
}

// PaymentProcessingFlow - Async payment handling
func (s *KafkaQueueService) PaymentProcessingFlow(rideID string, amount float64, userID string) error {
	_, err := s.Publish(QueuePayments, map[string]interface{}{
		"ride_id":     rideID,
		"amount":      amount,
		"user_id":     userID,
		"event":       "payment_initiated",
		"gateway":     "razorpay",
		"retry_count": 0,
	})
	return err
}

// NotificationFlow - Async notifications (SMS/Email/Push)
func (s *KafkaQueueService) NotificationFlow(userID string, notificationType string, content map[string]interface{}) error {
	_, err := s.Publish(QueueNotifications, map[string]interface{}{
		"user_id":  userID,
		"type":     notificationType,
		"content":  content,
		"channels": []string{"push", "sms"},
	})
	return err
}

// DriverLocationUpdate - High-frequency location updates
func (s *KafkaQueueService) DriverLocationUpdate(driverID string, lat, lng float64, timestamp int64) error {
	_, err := s.Publish(QueueDriverLocation, map[string]interface{}{
		"driver_id": driverID,
		"lat":       lat,
		"lng":       lng,
		"timestamp": timestamp,
		"event":     "location_update",
	})
	return err
}

// EmailSMSFlow - Async email/SMS sending
func (s *KafkaQueueService) EmailSMSFlow(recipient string, messageType string, template string, data map[string]interface{}) error {
	_, err := s.Publish(QueueEmailSMS, map[string]interface{}{
		"recipient": recipient,
		"type":      messageType,
		"template":  template,
		"data":      data,
		"provider":  "twilio", // or sendgrid
	})
	return err
}

// AnalyticsFlow - Async analytics events
func (s *KafkaQueueService) AnalyticsFlow(event string, properties map[string]interface{}) error {
	_, err := s.Publish(QueueAnalytics, map[string]interface{}{
		"event":      event,
		"properties": properties,
		"timestamp":  time.Now().Unix(),
	})
	return err
}

// GetQueueStatus returns queue metrics
func (s *KafkaQueueService) GetQueueStatus() map[string]interface{} {
	status := map[string]interface{}{}

	for name, queue := range s.queues {
		status[name] = map[string]interface{}{
			"length":      len(queue),
			"capacity":    cap(queue),
			"utilization": float64(len(queue)) / float64(cap(queue)) * 100,
		}
	}

	return status
}

// Message Flow Documentation
func GetKafkaMessageFlows() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"flow":  "Ride Created → Kafka → Matching → Driver Notify",
			"queue": QueueRideMatching,
			"steps": []string{
				"1. Ride created by rider",
				"2. Published to ride_matching queue",
				"3. Matching service consumes",
				"4. Driver notified via notifications queue",
			},
		},
		{
			"flow":  "Payment Initiated → Kafka → Payment Processing",
			"queue": QueuePayments,
			"steps": []string{
				"1. Payment initiated",
				"2. Published to payments queue",
				"3. Payment service processes async",
				"4. Webhook confirmation queued",
			},
		},
		{
			"flow":  "Driver Location → Kafka → Location Service",
			"queue": QueueDriverLocation,
			"steps": []string{
				"1. Driver location update received",
				"2. Published to driver_location queue",
				"3. Batch processed every 5 seconds",
				"4. Updated in Redis for real-time",
			},
		},
	}
}

var KafkaSvc = NewKafkaQueueService()

func GetKafkaQueueService() *KafkaQueueService {
	return KafkaSvc
}
