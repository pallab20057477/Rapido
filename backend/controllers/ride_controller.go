package controllers

import (
	"fmt"
	"net/http"
	"strconv"

	"rapido-backend/models"
	"rapido-backend/services"
	"rapido-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type RideController struct {
	Service *services.RideService
}

func NewRideController() *RideController {
	return &RideController{Service: services.NewRideService()}
}

// RequestRideRequest request body
type RequestRideRequest struct {
	VehicleType    string                 `json:"vehicle_type" binding:"required"`
	PickupLat      float64                `json:"pickup_lat" binding:"required"`
	PickupLng      float64                `json:"pickup_lng" binding:"required"`
	PickupAddress  string                 `json:"pickup_address" binding:"required"`
	DropoffLat     float64                `json:"dropoff_lat" binding:"required"`
	DropoffLng     float64                `json:"dropoff_lng" binding:"required"`
	DropoffAddress string                 `json:"dropoff_address" binding:"required"`
	PromoCode      string                 `json:"promo_code,omitempty"`
	PaymentMethod  string                 `json:"payment_method" binding:"required"`
	Preferences    models.RidePreferences `json:"preferences,omitempty"`
}

// RequestRide requests a new ride
func (c *RideController) RequestRide(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	userRole := ctx.GetString("userRole")

	utils.Debug("RequestRide initiated",
		zap.String("user_id", userID),
		zap.String("user_role", userRole))

	uid, err := uuid.Parse(userID)
	if err != nil || uid == uuid.Nil {
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", "invalid or missing user identity"))
		return
	}

	// Validate user role before proceeding
	if userRole != "rider" {
		ctx.JSON(http.StatusForbidden, utils.ErrorResponse("Forbidden", fmt.Sprintf("User role '%s' cannot request rides. Only riders can request rides.", userRole)))
		return
	}

	var req RequestRideRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	// Generate idempotency key
	idempotencyKey := ctx.GetHeader("Idempotency-Key")
	if idempotencyKey == "" {
		idempotencyKey = utils.GenerateIdempotencyKey()
	}

	ride, err := c.Service.RequestRide(services.RequestRideRequest{
		RiderID:        uid,
		VehicleType:    req.VehicleType,
		PickupLat:      req.PickupLat,
		PickupLng:      req.PickupLng,
		PickupAddress:  req.PickupAddress,
		DropoffLat:     req.DropoffLat,
		DropoffLng:     req.DropoffLng,
		DropoffAddress: req.DropoffAddress,
		PromoCode:      req.PromoCode,
		PaymentMethod:  req.PaymentMethod,
		Preferences:    req.Preferences,
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		utils.Warn("RequestRide failed", zap.String("user_id", uid.String()), zap.Error(err))
		ctx.JSON(http.StatusBadRequest, utils.SanitizedErrorResponse("Failed to request ride", err.Error()))
		return
	}

	ctx.JSON(http.StatusCreated, utils.SuccessResponse("Ride requested successfully", ride))
}

// AcceptRide accepts a ride (Driver)
func (c *RideController) AcceptRide(ctx *gin.Context) {
	driverID := ctx.GetString("userID")
	if driverID == "" {
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}
	did, err := uuid.Parse(driverID)
	if err != nil {
		utils.Warn("AcceptRide - invalid driver ID", zap.String("driver_id", driverID), zap.Error(err))
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	rideID := ctx.Param("id")
	rid, err := uuid.Parse(rideID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid ride ID", ""))
		return
	}

	ride, err := c.Service.AcceptRide(rid, did)
	if err != nil {
		utils.Warn("AcceptRide service error", zap.String("ride_id", rid.String()), zap.String("driver_id", did.String()), zap.Error(err))
		ctx.JSON(http.StatusBadRequest, utils.SanitizedErrorResponse("Failed to accept ride", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Ride accepted", ride))
}

// RejectRide rejects a ride (Driver)
func (c *RideController) RejectRide(ctx *gin.Context) {
	driverID := ctx.GetString("userID")
	if driverID == "" {
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}
	did, err := uuid.Parse(driverID)
	if err != nil {
		utils.Warn("RejectRide - invalid driver ID", zap.String("driver_id", driverID), zap.Error(err))
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	rideID := ctx.Param("id")
	rid, err := uuid.Parse(rideID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid ride ID", ""))
		return
	}

	if err := c.Service.RejectRide(rid, did, "driver_rejected"); err != nil {
		utils.Warn("RejectRide service error", zap.String("ride_id", rid.String()), zap.String("driver_id", did.String()), zap.Error(err))
		ctx.JSON(http.StatusBadRequest, utils.SanitizedErrorResponse("Failed to reject ride", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Ride rejected", nil))
}

// DriverArrived marks driver as arrived
func (c *RideController) DriverArrived(ctx *gin.Context) {
	driverID := ctx.GetString("userID")
	if driverID == "" {
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}
	did, err := uuid.Parse(driverID)
	if err != nil {
		utils.Warn("DriverArrived - invalid driver ID", zap.String("driver_id", driverID), zap.Error(err))
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	rideID := ctx.Param("id")
	rid, err := uuid.Parse(rideID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid ride ID", ""))
		return
	}

	ride, err := c.Service.DriverArrived(rid, did)
	if err != nil {
		utils.Warn("DriverArrived service error", zap.String("ride_id", rid.String()), zap.String("driver_id", did.String()), zap.Error(err))
		ctx.JSON(http.StatusBadRequest, utils.SanitizedErrorResponse("Failed to mark arrival", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Driver arrived", ride))
}

// StartRideRequest request body
type StartRideRequest struct {
	OTP string `json:"otp" binding:"required"`
}

// StartRide starts the ride
func (c *RideController) StartRide(ctx *gin.Context) {
	driverID := ctx.GetString("userID")
	if driverID == "" {
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}
	did, err := uuid.Parse(driverID)
	if err != nil {
		utils.Warn("StartRide - invalid driver ID", zap.String("driver_id", driverID), zap.Error(err))
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	rideID := ctx.Param("id")
	rid, err := uuid.Parse(rideID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid ride ID", ""))
		return
	}

	var req StartRideRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	ride, err := c.Service.StartRide(rid, did, req.OTP)
	if err != nil {
		utils.Warn("StartRide service error", zap.String("ride_id", rid.String()), zap.String("driver_id", did.String()), zap.Error(err))
		ctx.JSON(http.StatusBadRequest, utils.SanitizedErrorResponse("Failed to start ride", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Ride started", ride))
}

// RideUpdateLocationRequest request body
type RideUpdateLocationRequest struct {
	Lat float64 `json:"lat" binding:"required"`
	Lng float64 `json:"lng" binding:"required"`
}

// UpdateLocation updates ride location (Driver)
func (c *RideController) UpdateLocation(ctx *gin.Context) {
	rideID := ctx.Param("id")
	rid, err := uuid.Parse(rideID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid ride ID", ""))
		return
	}

	var req RideUpdateLocationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	if err := c.Service.UpdateRideLocation(rid, req.Lat, req.Lng); err != nil {
		utils.Warn("UpdateLocation service error", zap.String("ride_id", rid.String()), zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to update location", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Location updated", nil))
}

// CompleteRideRequest request body
type CompleteRideRequest struct {
	FinalLat float64 `json:"final_lat" binding:"required"`
	FinalLng float64 `json:"final_lng" binding:"required"`
}

// CompleteRide completes the ride
func (c *RideController) CompleteRide(ctx *gin.Context) {
	driverID := ctx.GetString("userID")
	if driverID == "" {
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}
	did, err := uuid.Parse(driverID)
	if err != nil {
		utils.Warn("CompleteRide - invalid driver ID", zap.String("driver_id", driverID), zap.Error(err))
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	rideID := ctx.Param("id")
	rid, err := uuid.Parse(rideID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid ride ID", ""))
		return
	}

	var req CompleteRideRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	ride, err := c.Service.CompleteRide(rid, did, req.FinalLat, req.FinalLng)
	if err != nil {
		utils.Warn("CompleteRide service error", zap.String("ride_id", rid.String()), zap.String("driver_id", did.String()), zap.Error(err))
		ctx.JSON(http.StatusBadRequest, utils.SanitizedErrorResponse("Failed to complete ride", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Ride completed", ride))
}

// CancelRideRequest request body
type CancelRideRequest struct {
	Reason string `json:"reason,omitempty"`
}

// UpdateRideStatusRequest request body for PATCH status updates
type UpdateRideStatusRequest struct {
	Status   string  `json:"status" binding:"required,oneof=accepted started completed cancelled"`
	OTP      string  `json:"otp,omitempty"`
	FinalLat float64 `json:"final_lat,omitempty"`
	FinalLng float64 `json:"final_lng,omitempty"`
}

// CancelRide cancels the ride
func (c *RideController) CancelRide(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	if userID == "" {
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		utils.Warn("CancelRide - invalid user ID", zap.String("user_id", userID), zap.Error(err))
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}
	role := ctx.GetString("userRole")

	rideID := ctx.Param("id")
	rid, err := uuid.Parse(rideID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid ride ID", ""))
		return
	}

	var req CancelRideRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		// allow empty body, but reject malformed JSON
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	isRider := role == "rider"
	ride, err := c.Service.CancelRide(rid, uid, req.Reason, isRider)
	if err != nil {
		utils.Warn("CancelRide service error", zap.String("ride_id", rid.String()), zap.String("user_id", uid.String()), zap.Error(err))
		ctx.JSON(http.StatusBadRequest, utils.SanitizedErrorResponse("Failed to cancel ride", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Ride cancelled", ride))
}

// GetRide gets ride details
func (c *RideController) GetRide(ctx *gin.Context) {
	rideID := ctx.Param("id")
	rid, err := uuid.Parse(rideID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid ride ID", ""))
		return
	}

	ride, err := c.Service.GetRide(rid)
	if err != nil {
		utils.Warn("GetRide - not found", zap.String("ride_id", rid.String()), zap.Error(err))
		ctx.JSON(http.StatusNotFound, utils.SanitizedErrorResponse("Ride not found", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Ride retrieved", ride))
}

// GetActiveRide gets active ride for user
func (c *RideController) GetActiveRide(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	if userID == "" {
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		utils.Warn("GetActiveRide - invalid user ID", zap.String("user_id", userID), zap.Error(err))
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}
	role := ctx.GetString("userRole")

	var ride *models.Ride
	var rideErr error

	if role == "driver" {
		ride, rideErr = c.Service.GetActiveRideForDriver(uid)
	} else {
		ride, rideErr = c.Service.GetActiveRideForRider(uid)
	}

	if rideErr != nil {
		ctx.JSON(http.StatusNotFound, utils.ErrorResponse("No active ride", ""))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Active ride found", ride))
}

// GetRideHistory gets ride history
func (c *RideController) GetRideHistory(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	if userID == "" {
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		utils.Warn("GetRideHistory - invalid user ID", zap.String("user_id", userID), zap.Error(err))
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}
	role := ctx.GetString("userRole")

	page, err := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}
	perPage, err := strconv.Atoi(ctx.DefaultQuery("per_page", "10"))
	if err != nil || perPage < 1 {
		perPage = 10
	}
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 10
	}
	if perPage > 100 {
		perPage = 100
	}

	rides, total, err := c.Service.GetRideHistory(uid, role, page, perPage)
	if err != nil {
		utils.Warn("GetRideHistory - service error", zap.String("user_id", uid.String()), zap.String("role", role), zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to fetch history", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.PaginatedResponse(rides, page, perPage, total))
}

// EstimateFareRequest query params
type EstimateFareRequest struct {
	PickupLat   float64 `form:"pickup_lat" binding:"required"`
	PickupLng   float64 `form:"pickup_lng" binding:"required"`
	DropoffLat  float64 `form:"dropoff_lat" binding:"required"`
	DropoffLng  float64 `form:"dropoff_lng" binding:"required"`
	VehicleType string  `form:"vehicle_type" binding:"required"`
}

// EstimateFare estimates ride fare
func (c *RideController) EstimateFare(ctx *gin.Context) {
	var req EstimateFareRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid parameters", err.Error()))
		return
	}

	estimate, err := c.Service.EstimateFare(
		req.PickupLat, req.PickupLng,
		req.DropoffLat, req.DropoffLng,
		req.VehicleType,
	)
	if err != nil {
		utils.Warn("EstimateFare service error", zap.Error(err))
		ctx.JSON(http.StatusBadRequest, utils.SanitizedErrorResponse("Failed to estimate", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Fare estimated", estimate))
}

// GetNearbyDrivers gets nearby drivers
func (c *RideController) GetNearbyDrivers(ctx *gin.Context) {
	lat, err := strconv.ParseFloat(ctx.Query("lat"), 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid latitude", ""))
		return
	}
	lng, err := strconv.ParseFloat(ctx.Query("lng"), 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid longitude", ""))
		return
	}
	vehicleType := ctx.Query("vehicle_type")

	if lat == 0 || lng == 0 {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Latitude and longitude required", ""))
		return
	}

	drivers, err := c.Service.GetNearbyDrivers(lat, lng, vehicleType)
	if err != nil {
		utils.Warn("GetNearbyDrivers service error", zap.Float64("lat", lat), zap.Float64("lng", lng), zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to fetch drivers", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Nearby drivers", drivers))
}

// UpdateRideStatus handles PATCH /rides/:id/status (RESTful status update)
func (c *RideController) UpdateRideStatus(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	if userID == "" {
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		utils.Warn("UpdateRideStatus - invalid user ID", zap.String("user_id", userID), zap.Error(err))
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}
	role := ctx.GetString("userRole")

	rideID := ctx.Param("id")
	rid, err := uuid.Parse(rideID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid ride ID", ""))
		return
	}

	var req UpdateRideStatusRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	// Route to appropriate service method based on status
	var ride interface{}
	var updateErr error

	switch req.Status {
	case "accepted":
		// Driver accepting via status endpoint
		if role != "driver" {
			ctx.JSON(http.StatusForbidden, utils.ErrorResponse("Forbidden", "only drivers can accept rides"))
			return
		}
		ride, updateErr = c.Service.AcceptRide(rid, uid)

	case "started":
		// Only drivers can mark a ride as started and must provide OTP
		if role != "driver" {
			ctx.JSON(http.StatusForbidden, utils.ErrorResponse("Forbidden", "only drivers can start rides"))
			return
		}
		if req.OTP == "" {
			ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", "otp is required to start ride"))
			return
		}
		ride, updateErr = c.Service.StartRide(rid, uid, req.OTP)
	case "completed":
		// Only drivers can complete rides; final coordinates optional
		if role != "driver" {
			ctx.JSON(http.StatusForbidden, utils.ErrorResponse("Forbidden", "only drivers can complete rides"))
			return
		}
		ride, updateErr = c.Service.CompleteRide(rid, uid, req.FinalLat, req.FinalLng)
	case "cancelled":
		isRider := role == "rider"
		ride, updateErr = c.Service.CancelRide(rid, uid, "cancelled_via_api", isRider)
	default:
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid status transition", ""))
		return
	}

	if updateErr != nil {
		utils.Warn("UpdateRideStatus service error", zap.String("ride_id", rid.String()), zap.String("user_id", uid.String()), zap.String("status", req.Status), zap.Error(updateErr))
		ctx.JSON(http.StatusBadRequest, utils.SanitizedErrorResponse("Failed to update status", updateErr.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Status updated", ride))
}

// TrackRide provides real-time ride tracking for riders
func (c *RideController) TrackRide(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	if userID == "" {
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		utils.Warn("TrackRide - invalid user ID", zap.String("user_id", userID), zap.Error(err))
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	rideID := ctx.Param("id")
	rid, err := uuid.Parse(rideID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid ride ID", ""))
		return
	}

	// Get ride with driver location
	tracking, err := c.Service.TrackRide(rid, uid)
	if err != nil {
		utils.Warn("TrackRide - not found or inaccessible", zap.String("ride_id", rid.String()), zap.String("user_id", uid.String()), zap.Error(err))
		ctx.JSON(http.StatusNotFound, utils.SanitizedErrorResponse("Ride not found or not accessible", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Ride tracking", tracking))
}

// GetRideETA calculates ETA for ride completion
func (c *RideController) GetRideETA(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	if userID == "" {
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		utils.Warn("GetRideETA - invalid user ID", zap.String("user_id", userID), zap.Error(err))
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	rideID := ctx.Param("id")
	rid, err := uuid.Parse(rideID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid ride ID", ""))
		return
	}

	eta, err := c.Service.GetRideETA(rid, uid)
	if err != nil {
		utils.Warn("GetRideETA service error", zap.String("ride_id", rid.String()), zap.String("user_id", uid.String()), zap.Error(err))
		ctx.JSON(http.StatusBadRequest, utils.SanitizedErrorResponse("Failed to calculate ETA", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("ETA calculated", eta))
}

// RetryMatch retries driver matching for a ride
func (c *RideController) RetryMatch(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	if userID == "" {
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		utils.Warn("RetryMatch - invalid user ID", zap.String("user_id", userID), zap.Error(err))
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	rideID := ctx.Param("id")
	rid, err := uuid.Parse(rideID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid ride ID", ""))
		return
	}

	// Retry matching
	result, err := c.Service.RetryMatch(rid, uid)
	if err != nil {
		utils.Warn("RetryMatch service error", zap.String("ride_id", rid.String()), zap.String("user_id", uid.String()), zap.Error(err))
		ctx.JSON(http.StatusBadRequest, utils.SanitizedErrorResponse("Failed to retry matching", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Matching retried", result))
}

// ApplyPromoCode handles POST /rides/:id/apply-promo
func (c *RideController) ApplyPromoCode(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	if userID == "" {
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		utils.Warn("ApplyPromoCode - invalid user ID", zap.String("user_id", userID), zap.Error(err))
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	rideID := ctx.Param("id")
	rid, err := uuid.Parse(rideID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid ride ID", ""))
		return
	}

	var req struct {
		PromoCode string `json:"promo_code" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	// Apply promo code to ride
	result, err := c.Service.ApplyPromoCode(rid, uid, req.PromoCode)
	if err != nil {
		utils.Warn("ApplyPromoCode service error", zap.String("ride_id", rid.String()), zap.String("user_id", uid.String()), zap.Error(err))
		ctx.JSON(http.StatusBadRequest, utils.SanitizedErrorResponse("Failed to apply promo code", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Promo code applied", result))
}

// GetFareBreakdown handles GET /rides/:id/fare
func (c *RideController) GetFareBreakdown(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	if userID == "" {
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		utils.Warn("GetFareBreakdown - invalid user ID", zap.String("user_id", userID), zap.Error(err))
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	rideID := ctx.Param("id")
	rid, err := uuid.Parse(rideID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid ride ID", ""))
		return
	}

	// Get fare breakdown
	breakdown, err := c.Service.GetFareBreakdown(rid, uid)
	if err != nil {
		utils.Warn("GetFareBreakdown - not found", zap.String("ride_id", rid.String()), zap.String("user_id", uid.String()), zap.Error(err))
		ctx.JSON(http.StatusNotFound, utils.SanitizedErrorResponse("Ride not found", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Fare breakdown retrieved", breakdown))
}

// GetMatchStatus handles GET /rides/:id/match-status (admin/debug visibility)
func (c *RideController) GetMatchStatus(ctx *gin.Context) {
	rideID := ctx.Param("id")
	rid, err := uuid.Parse(rideID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid ride ID", ""))
		return
	}

	// Get matching status
	status, err := c.Service.GetMatchStatus(rid)
	if err != nil {
		utils.Warn("GetMatchStatus - not found", zap.String("ride_id", rid.String()), zap.Error(err))
		ctx.JSON(http.StatusNotFound, utils.SanitizedErrorResponse("Ride not found", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Match status retrieved", status))
}

// ReassignRide handles POST /rides/:id/reassign (manual/auto reassignment)
func (c *RideController) ReassignRide(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	if userID == "" {
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		utils.Warn("ReassignRide - invalid user ID", zap.String("user_id", userID), zap.Error(err))
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	rideID := ctx.Param("id")
	rid, err := uuid.Parse(rideID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid ride ID", ""))
		return
	}

	var req struct {
		Reason               string   `json:"reason"`
		PreferredDriverTypes []string `json:"preferred_driver_types"`
		Priority             string   `json:"priority"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	// Reassign ride to new driver
	result, err := c.Service.ReassignRide(rid, uid, req.Reason, req.PreferredDriverTypes, req.Priority)
	if err != nil {
		utils.Warn("ReassignRide service error", zap.String("ride_id", rid.String()), zap.String("user_id", uid.String()), zap.Error(err))
		ctx.JSON(http.StatusBadRequest, utils.SanitizedErrorResponse("Failed to reassign ride", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Ride reassigned", result))
}

// GetFailureReason handles GET /rides/:id/failure-reason (admin debug)
func (c *RideController) GetFailureReason(ctx *gin.Context) {
	rideID := ctx.Param("id")
	rid, err := uuid.Parse(rideID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid ride ID", ""))
		return
	}

	// Get failure analysis
	analysis, err := c.Service.GetFailureReason(rid)
	if err != nil {
		utils.Warn("GetFailureReason - not found", zap.String("ride_id", rid.String()), zap.Error(err))
		ctx.JSON(http.StatusNotFound, utils.SanitizedErrorResponse("Ride not found", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Failure analysis retrieved", analysis))
}
