package services

import (
	"context"
	"fmt"
	"time"

	"rapido-backend/database"
	"rapido-backend/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Saga pattern for distributed transaction across services
type RideBookingSaga struct {
	db            *gorm.DB
	eventBus      *EventBus
	safePayment   *SafePaymentService
	driverService *DriverService

	sagaID      uuid.UUID
	rideID      uuid.UUID
	riderID     uuid.UUID
	driverID    uuid.UUID
	amount      float64
	steps       []SagaStep
	currentStep int
	status      SagaStatus
}

type SagaStatus string

const (
	SagaPending      SagaStatus = "PENDING"
	SagaInProgress   SagaStatus = "IN_PROGRESS"
	SagaCompleted    SagaStatus = "COMPLETED"
	SagaFailed       SagaStatus = "FAILED"
	SagaCompensating SagaStatus = "COMPENSATING"
)

type StepStatus string

const (
	StepPending     StepStatus = "PENDING"
	StepCompleted   StepStatus = "COMPLETED"
	StepFailed      StepStatus = "FAILED"
	StepCompensated StepStatus = "COMPENSATED"
)

type SagaStep struct {
	Name       string
	Execute    func() error
	Compensate func() error
	Status     StepStatus
}

type SagaLog struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key"`
	SagaID    uuid.UUID `gorm:"type:uuid;index"`
	StepName  string
	Status    string
	ErrorMsg  string
	CreatedAt time.Time
}

func NewRideBookingSaga(rideID, riderID, driverID uuid.UUID, amount float64) *RideBookingSaga {
	return &RideBookingSaga{
		db:            database.DB,
		eventBus:      EventBusInstance,
		safePayment:   SafePaymentServiceInstance,
		driverService: NewDriverService(),

		sagaID:   uuid.New(),
		rideID:   rideID,
		riderID:  riderID,
		driverID: driverID,
		amount:   amount,
		steps:    []SagaStep{},
		status:   SagaPending,
	}
}

func (s *RideBookingSaga) Execute() error {
	s.status = SagaInProgress
	s.currentStep = 0

	// Define saga steps
	s.steps = []SagaStep{
		{
			Name: "RESERVE_DRIVER",
			Execute: func() error {
				// Lock driver - prevents other rides from assigning
				return s.reserveDriver()
			},
			Compensate: func() error {
				// Release driver lock
				return s.releaseDriver()
			},
		},
		{
			Name: "HOLD_PAYMENT",
			Execute: func() error {
				// Pre-authorize payment
				_, err := s.safePayment.ProcessPayment(context.Background(), PaymentRequest{
					RideID:         s.rideID,
					UserID:         s.riderID,
					Amount:         s.amount,
					Method:         "card",
					IdempotencyKey: fmt.Sprintf("ride:%s:payment", s.rideID),
				})
				return err
			},
			Compensate: func() error {
				// Release payment hold
				// In real impl: call payment gateway void
				return nil
			},
		},
		{
			Name: "CREATE_RIDE",
			Execute: func() error {
				// Create ride record
				ride := models.Ride{
					ID:       s.rideID,
					RiderID:  s.riderID,
					DriverID: &s.driverID,
					Status:   models.RideStatusDriverAssigned,
				}
				return s.db.Create(&ride).Error
			},
			Compensate: func() error {
				// Cancel ride
				return s.db.Model(&models.Ride{}).Where("id = ?", s.rideID).
					Update("status", models.RideStatusCancelled).Error
			},
		},
		{
			Name: "NOTIFY_DRIVER",
			Execute: func() error {
				// Send notification
				if s.eventBus != nil {
					s.eventBus.PublishRideAccepted(s.rideID, s.driverID, s.riderID, s.amount)
				}
				return nil
			},
			Compensate: func() error {
				// No compensation needed for notification
				return nil
			},
		},
	}

	// Execute steps
	for i, step := range s.steps {
		s.currentStep = i
		s.logStep(step.Name, StepPending)

		if err := step.Execute(); err != nil {
			step.Status = StepFailed
			s.logStep(step.Name, StepFailed, err.Error())

			// Compensate previous steps
			s.compensate(i - 1)
			s.status = SagaFailed
			return fmt.Errorf("saga failed at step %s: %w", step.Name, err)
		}

		step.Status = StepCompleted
		s.logStep(step.Name, StepCompleted)
	}

	s.status = SagaCompleted
	s.logSagaCompletion()
	return nil
}

func (s *RideBookingSaga) compensate(lastCompletedIndex int) {
	s.status = SagaCompensating

	for i := lastCompletedIndex; i >= 0; i-- {
		step := s.steps[i]
		if step.Status == StepCompleted {
			if err := step.Compensate(); err != nil {
				// Log compensation failure - needs manual intervention
				s.logStep(step.Name, StepFailed, "compensation failed: "+err.Error())
			} else {
				step.Status = StepCompensated
				s.logStep(step.Name, StepCompensated)
			}
		}
	}
}

func (s *RideBookingSaga) reserveDriver() error {
	// Set driver as assigned in Redis with TTL
	lockKey := fmt.Sprintf("driver:lock:%s", s.driverID)
	acquired, err := database.RedisClient.SetNX(context.Background(), lockKey, s.rideID.String(), 5*time.Minute).Result()
	if err != nil {
		return err
	}
	if !acquired {
		return fmt.Errorf("driver already assigned to another ride")
	}

	// Update driver status in DB
	return s.db.Model(&models.Driver{}).Where("id = ?", s.driverID).
		Update("current_ride_id", s.rideID).Error
}

func (s *RideBookingSaga) releaseDriver() error {
	lockKey := fmt.Sprintf("driver:lock:%s", s.driverID)
	database.RedisClient.Del(context.Background(), lockKey)

	return s.db.Model(&models.Driver{}).Where("id = ?", s.driverID).
		Update("current_ride_id", nil).Error
}

func (s *RideBookingSaga) logStep(stepName string, status StepStatus, errorMsg ...string) {
	log := SagaLog{
		ID:        uuid.New(),
		SagaID:    s.sagaID,
		StepName:  stepName,
		Status:    string(status),
		CreatedAt: time.Now(),
	}
	if len(errorMsg) > 0 {
		log.ErrorMsg = errorMsg[0]
	}
	s.db.Create(&log)
}

func (s *RideBookingSaga) logSagaCompletion() {
	if s.eventBus != nil {
		s.eventBus.Publish("saga.completed", map[string]interface{}{
			"saga_id": s.sagaID.String(),
			"ride_id": s.rideID.String(),
			"status":  string(s.status),
			"steps":   len(s.steps),
		}, nil)
	}
}
