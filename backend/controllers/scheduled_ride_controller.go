package controllers

import (
	"net/http"
	"time"

	"rapido-backend/services"
	"rapido-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ScheduledRideController handles scheduled ride requests
type ScheduledRideController struct {
	service *services.ScheduledRideService
}

// NewScheduledRideController creates new controller
func NewScheduledRideController() *ScheduledRideController {
	return &ScheduledRideController{
		service: services.NewScheduledRideService(),
	}
}

// ScheduleRideRequest represents request to schedule a ride
type ScheduleRideRequest struct {
	VehicleType    string    `json:"vehicle_type" binding:"required"`
	PickupLat      float64   `json:"pickup_lat" binding:"required"`
	PickupLng      float64   `json:"pickup_lng" binding:"required"`
	PickupAddress  string    `json:"pickup_address" binding:"required"`
	DropoffLat     float64   `json:"dropoff_lat" binding:"required"`
	DropoffLng     float64   `json:"dropoff_lng" binding:"required"`
	DropoffAddress string    `json:"dropoff_address" binding:"required"`
	ScheduledAt    time.Time `json:"scheduled_at" binding:"required"`
	Notes          string    `json:"notes"`
	Preferences    struct {
		FemaleDriverOnly bool `json:"female_driver_only"`
		ACRequired       bool `json:"ac_required"`
		LuggageSpace     bool `json:"luggage_space"`
	} `json:"preferences"`
}

// ScheduleRide schedules a future ride
// POST /api/v1/rides/schedule
func (c *ScheduledRideController) ScheduleRide(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	uid, _ := uuid.Parse(userID)

	var req ScheduleRideRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	scheduledRide, err := c.service.ScheduleRide(
		uid,
		req.PickupLat,
		req.PickupLng,
		req.PickupAddress,
		req.DropoffLat,
		req.DropoffLng,
		req.DropoffAddress,
		req.VehicleType,
		req.ScheduledAt,
		req.Notes,
	)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Failed to schedule ride", err.Error()))
		return
	}

	ctx.JSON(http.StatusCreated, utils.SuccessResponse("Ride scheduled successfully", scheduledRide))
}

// GetScheduledRides gets upcoming scheduled rides
// GET /api/v1/rides/scheduled
func (c *ScheduledRideController) GetScheduledRides(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	uid, _ := uuid.Parse(userID)

	rides, err := c.service.GetScheduledRides(uid)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to get scheduled rides", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Scheduled rides retrieved", rides))
}

// CancelScheduledRide cancels a scheduled ride
// POST /api/v1/rides/scheduled/:id/cancel
func (c *ScheduledRideController) CancelScheduledRide(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	uid, _ := uuid.Parse(userID)

	rideID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid ride ID", err.Error()))
		return
	}

	if err := c.service.CancelScheduledRide(rideID, uid); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Failed to cancel scheduled ride", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Scheduled ride cancelled successfully", nil))
}

// UpdateScheduledRide updates a scheduled ride
// PUT /api/v1/rides/scheduled/:id
func (c *ScheduledRideController) UpdateScheduledRide(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	uid, _ := uuid.Parse(userID)

	rideID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid ride ID", err.Error()))
		return
	}

	var req ScheduleRideRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	// Update via service
	updatedRide, err := c.service.UpdateScheduledRide(rideID, uid, req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Failed to update scheduled ride", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Scheduled ride updated successfully", updatedRide))
}

// GetScheduledRideDetails gets details of a specific scheduled ride
// GET /api/v1/rides/scheduled/:id
func (c *ScheduledRideController) GetScheduledRideDetails(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	uid, _ := uuid.Parse(userID)

	rideID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid ride ID", err.Error()))
		return
	}

	ride, err := c.service.GetScheduledRideByID(rideID, uid)
	if err != nil {
		ctx.JSON(http.StatusNotFound, utils.ErrorResponse("Scheduled ride not found", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Scheduled ride details retrieved", ride))
}
