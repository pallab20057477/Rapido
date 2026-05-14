package services

import (
	"fmt"
	"log"
	"time"

	"rapido-backend/database"
	"rapido-backend/models"

	"github.com/google/uuid"
)

type RideState string

const (
	RideStatusRequested RideState = "requested"
	RideStatusAccepted  RideState = "accepted"
	RideStatusArrived   RideState = "arrived"
	RideStatusStarted   RideState = "started"
	RideStatusCompleted RideState = "completed"
	RideStatusCancelled RideState = "cancelled"
	RideStatusNoDrivers RideState = "no_drivers"
	RideStatusExpired   RideState = "expired"
)

type StateTransition struct {
	From RideState
	To   RideState
}

type StateMachine struct {
	validTransitions map[RideState][]RideState
}

func NewRideStateMachine() *StateMachine {
	sm := &StateMachine{
		validTransitions: make(map[RideState][]RideState),
	}

	// Define valid transitions
	sm.validTransitions[RideStatusRequested] = []RideState{
		RideStatusAccepted,
		RideStatusCancelled,
		RideStatusNoDrivers,
		RideStatusExpired,
	}

	sm.validTransitions[RideStatusAccepted] = []RideState{
		RideStatusArrived,
		RideStatusCancelled,
	}

	sm.validTransitions[RideStatusArrived] = []RideState{
		RideStatusStarted,
		RideStatusCancelled,
	}

	sm.validTransitions[RideStatusStarted] = []RideState{
		RideStatusCompleted,
		RideStatusCancelled,
	}

	sm.validTransitions[RideStatusCompleted] = []RideState{} // Terminal state
	sm.validTransitions[RideStatusCancelled] = []RideState{} // Terminal state
	sm.validTransitions[RideStatusNoDrivers] = []RideState{} // Terminal state
	sm.validTransitions[RideStatusExpired] = []RideState{}   // Terminal state

	return sm
}

func (sm *StateMachine) IsValidTransition(from, to RideState) bool {
	validStates, exists := sm.validTransitions[from]
	if !exists {
		return false
	}

	for _, validState := range validStates {
		if validState == to {
			return true
		}
	}

	return false
}

func (sm *StateMachine) GetValidTransitions(from RideState) []RideState {
	transitions, exists := sm.validTransitions[from]
	if !exists {
		return []RideState{}
	}

	// Return a copy to prevent modification
	result := make([]RideState, len(transitions))
	copy(result, transitions)
	return result
}

func (sm *StateMachine) CanCancel(ride *models.Ride) bool {
	switch RideState(ride.Status) {
	case RideStatusRequested, RideStatusAccepted, RideStatusArrived:
		return true
	default:
		return false
	}
}

func (sm *StateMachine) IsTerminalState(ride *models.Ride) bool {
	switch RideState(ride.Status) {
	case RideStatusCompleted, RideStatusCancelled, RideStatusNoDrivers, RideStatusExpired:
		return true
	default:
		return false
	}
}

type RideStateMachineService struct {
	stateMachine *StateMachine
}

func NewRideStateMachineService() *RideStateMachineService {
	return &RideStateMachineService{
		stateMachine: NewRideStateMachine(),
	}
}

func (rms *RideStateMachineService) TransitionRide(ride *models.Ride, newStatus RideState, reason string) error {
	currentStatus := RideState(ride.Status)

	// Check if transition is valid
	if !rms.stateMachine.IsValidTransition(currentStatus, newStatus) {
		log.Printf("Invalid ride state transition: %s -> %s for ride %d",
			currentStatus, newStatus, ride.ID)
		return fmt.Errorf("invalid state transition from %s to %s", currentStatus, newStatus)
	}

	// Log the transition to RideStatusLog
	statusLog := &models.RideStatusLog{
		RideID:     ride.ID,
		FromStatus: string(currentStatus),
		ToStatus:   string(newStatus),
		Reason:     reason,
		ActorType:  "system", // Can be overridden by caller if needed
	}
	if err := database.DB.Create(statusLog).Error; err != nil {
		// Log error but don't fail the transition
		log.Printf("[WARN] Failed to create ride status log for ride %s: %v", ride.ID, err)
	}

	log.Printf("Ride %s transition: %s -> %s (reason: %s)",
		ride.ID, currentStatus, newStatus, reason)

	// Update ride status
	ride.Status = string(newStatus)

	// Set timestamps based on status
	now := time.Now()
	switch newStatus {
	case RideStatusAccepted:
		if ride.AcceptedAt == nil {
			ride.AcceptedAt = &now
		}
	case RideStatusArrived:
		if ride.ArrivedAt == nil {
			ride.ArrivedAt = &now
		}
	case RideStatusStarted:
		if ride.StartedAt == nil {
			ride.StartedAt = &now
		}
	case RideStatusCompleted:
		if ride.CompletedAt == nil {
			ride.CompletedAt = &now
		}
	case RideStatusCancelled:
		if ride.CancellationTime == nil {
			ride.CancellationTime = &now
		}
	}

	log.Printf("Ride %d transitioned: %s -> %s (reason: %s)",
		ride.ID, currentStatus, newStatus, reason)

	return nil
}

func (rms *RideStateMachineService) CanAcceptRide(ride *models.Ride) error {
	if RideState(ride.Status) != RideStatusRequested {
		return fmt.Errorf("ride cannot be accepted in current state: %s", ride.Status)
	}

	if ride.DriverID != nil {
		return fmt.Errorf("ride already has a driver assigned")
	}

	return nil
}

func (rms *RideStateMachineService) CanStartRide(ride *models.Ride) error {
	if RideState(ride.Status) != RideStatusArrived {
		return fmt.Errorf("ride cannot be started in current state: %s", ride.Status)
	}

	if ride.DriverID == nil {
		return fmt.Errorf("ride has no driver assigned")
	}

	return nil
}

func (rms *RideStateMachineService) CanCompleteRide(ride *models.Ride) error {
	if RideState(ride.Status) != RideStatusStarted {
		return fmt.Errorf("ride cannot be completed in current state: %s", ride.Status)
	}

	if ride.DriverID == nil {
		return fmt.Errorf("ride has no driver assigned")
	}

	return nil
}

func (rms *RideStateMachineService) CanCancelRide(ride *models.Ride, userID uuid.UUID, isDriver bool) error {
	if !rms.stateMachine.CanCancel(ride) {
		return fmt.Errorf("ride cannot be cancelled in current state: %s", ride.Status)
	}

	// Check who can cancel
	if isDriver {
		// Driver can cancel if they're assigned
		if ride.DriverID == nil || *ride.DriverID != userID {
			return fmt.Errorf("driver is not assigned to this ride")
		}
	} else {
		// User can cancel if they own the ride
		if ride.RiderID != userID {
			return fmt.Errorf("user is not the owner of this ride")
		}
	}

	return nil
}

func (rms *RideStateMachineService) GetRideTimeline(rideID uuid.UUID) (*models.RideTimelineResponse, error) {
	// Get all status logs for the ride
	var logs []models.RideStatusLog
	if err := database.DB.Where("ride_id = ?", rideID).
		Order("created_at ASC").
		Find(&logs).Error; err != nil {
		return nil, err
	}

	// Get ride details for current status
	var ride models.Ride
	if err := database.DB.First(&ride, rideID).Error; err != nil {
		return nil, err
	}

	// Build timeline entries
	entries := make([]models.RideTimelineEntry, len(logs))
	for i, log := range logs {
		entry := models.RideTimelineEntry{
			Status:      log.ToStatus,
			Timestamp:   log.CreatedAt,
			Actor:       log.ActorType,
			Description: log.Reason,
		}
		if log.LocationLat != nil && log.LocationLng != nil {
			entry.Location = &struct {
				Lat float64 `json:"lat"`
				Lng float64 `json:"lng"`
			}{
				Lat: *log.LocationLat,
				Lng: *log.LocationLng,
			}
		}
		entries[i] = entry
	}

	return &models.RideTimelineResponse{
		RideID:        rideID,
		CreatedAt:     ride.CreatedAt,
		Entries:       entries,
		CurrentStatus: ride.Status,
	}, nil
}

func (rms *RideStateMachineService) AutoExpireRide(ride *models.Ride) error {
	if RideState(ride.Status) != RideStatusRequested {
		return fmt.Errorf("ride cannot be auto-expired in current state: %s", ride.Status)
	}

	return rms.TransitionRide(ride, RideStatusExpired, "Auto-expired due to timeout")
}

func (rms *RideStateMachineService) AutoCancelNoDrivers(ride *models.Ride) error {
	if RideState(ride.Status) != RideStatusRequested {
		return fmt.Errorf("ride cannot be auto-cancelled in current state: %s", ride.Status)
	}

	return rms.TransitionRide(ride, RideStatusNoDrivers, "Auto-cancelled - no drivers available")
}

// GetStateDescription returns a human-readable description of the ride state
func GetStateDescription(state RideState) string {
	switch state {
	case RideStatusRequested:
		return "Looking for drivers"
	case RideStatusAccepted:
		return "Driver on the way"
	case RideStatusArrived:
		return "Driver has arrived"
	case RideStatusStarted:
		return "Ride in progress"
	case RideStatusCompleted:
		return "Ride completed"
	case RideStatusCancelled:
		return "Ride cancelled"
	case RideStatusNoDrivers:
		return "No drivers available"
	case RideStatusExpired:
		return "Request expired"
	default:
		return "Unknown status"
	}
}

// GetEstimatedTimeToArrival estimates time based on ride state
func GetEstimatedTimeToArrival(ride *models.Ride) *int {
	switch RideState(ride.Status) {
	case RideStatusRequested:
		// Estimate based on nearby drivers
		return nil // Unknown until driver accepts
	case RideStatusAccepted:
		// Estimate based on driver distance to pickup
		if ride.AcceptedAt != nil {
			// Rough estimate: 10-15 minutes
			eta := 12
			return &eta
		}
	case RideStatusArrived:
		// Driver is at pickup location
		eta := 1
		return &eta
	case RideStatusStarted:
		// Ride is in progress
		if ride.StartedAt != nil && ride.EstimatedDuration > 0 {
			elapsed := time.Since(*ride.StartedAt).Minutes()
			remaining := float64(ride.EstimatedDuration) - elapsed
			if remaining > 0 {
				eta := int(remaining)
				return &eta
			}
		}
	}

	return nil
}
