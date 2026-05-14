package controllers

import (
	"net/http"
	"strconv"
	"time"

	"rapido-backend/services"
	"rapido-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type DriverController struct {
	Service *services.DriverService
}

func NewDriverController() *DriverController {
	return &DriverController{Service: services.NewDriverService()}
}

// RegisterDriverRequest request body
type RegisterDriverRequest struct {
	LicenseNumber      string   `json:"license_number" binding:"required"`
	LicenseImage       string   `json:"license_image" binding:"required"`
	LicenseExpiry      string   `json:"license_expiry,omitempty"`
	RCNumber           string   `json:"rc_number" binding:"required"`
	RCImage            string   `json:"rc_image" binding:"required"`
	AadhaarNumber      string   `json:"aadhaar_number" binding:"required"`
	AadhaarImage       string   `json:"aadhaar_image" binding:"required"`
	VehicleType        string   `json:"vehicle_type" binding:"required"`
	VehicleMake        string   `json:"vehicle_make"`
	VehicleModel       string   `json:"vehicle_model"`
	VehicleYear        int      `json:"vehicle_year"`
	VehicleColor       string   `json:"vehicle_color"`
	VehicleNumberPlate string   `json:"vehicle_number_plate" binding:"required"`
	FuelType           string   `json:"fuel_type"`
	VehicleImage       string   `json:"vehicle_image"`
	Languages          []string `json:"languages"`
}

// RegisterDriver registers a new driver
func (c *DriverController) RegisterDriver(ctx *gin.Context) {
	userID := ctx.GetString("userID")

	if userID == "" {
		utils.Warn("RegisterDriver - empty user ID")
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	uid, err := uuid.Parse(userID)
	if err != nil {
		utils.Warn("RegisterDriver - invalid user ID format", zap.String("user_id", userID), zap.Error(err))
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	var req RegisterDriverRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	// Parse license expiry if provided
	var licenseExpiry *time.Time
	if req.LicenseExpiry != "" {
		// try YYYY-MM-DD then RFC3339
		if t, err := time.Parse("2006-01-02", req.LicenseExpiry); err == nil {
			licenseExpiry = &t
		} else if t2, err2 := time.Parse(time.RFC3339, req.LicenseExpiry); err2 == nil {
			licenseExpiry = &t2
		} else {
			ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", "invalid license_expiry format"))
			return
		}
	}

	driver, err := c.Service.RegisterDriver(uid, services.RegisterDriverRequest{
		LicenseNumber:      req.LicenseNumber,
		LicenseImage:       req.LicenseImage,
		LicenseExpiry:      licenseExpiry,
		RCNumber:           req.RCNumber,
		RCImage:            req.RCImage,
		AadhaarNumber:      req.AadhaarNumber,
		AadhaarImage:       req.AadhaarImage,
		VehicleType:        req.VehicleType,
		VehicleMake:        req.VehicleMake,
		VehicleModel:       req.VehicleModel,
		VehicleYear:        req.VehicleYear,
		VehicleColor:       req.VehicleColor,
		VehicleNumberPlate: req.VehicleNumberPlate,
		FuelType:           req.FuelType,
		VehicleImage:       req.VehicleImage,
		Languages:          req.Languages,
	})
	if err != nil {
		utils.Warn("RegisterDriver - registration failed", zap.String("user_id", uid.String()), zap.Error(err))
		ctx.JSON(http.StatusBadRequest, utils.SanitizedErrorResponse("Registration failed", err.Error()))
		return
	}

	ctx.JSON(http.StatusCreated, utils.SuccessResponse("Driver registered successfully", driver))
}

// GetDriverProfile gets driver profile
func (c *DriverController) GetDriverProfile(ctx *gin.Context) {
	userID := ctx.GetString("userID")

	// Validate userID is not empty
	if userID == "" {
		utils.Warn("GetDriverProfile - empty user ID")
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	// Parse UUID with error handling
	uid, err := uuid.Parse(userID)
	if err != nil {
		utils.Warn("GetDriverProfile - invalid user ID format",
			zap.String("user_id", userID),
			zap.Error(err))
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	// Get driver profile
	driver, err := c.Service.GetDriverProfile(uid)
	if err != nil {
		utils.Warn("GetDriverProfile - driver not found",
			zap.String("user_id", uid.String()),
			zap.Error(err))
		ctx.JSON(http.StatusNotFound, utils.SanitizedErrorResponse("Driver profile not found", err.Error()))
		return
	}

	utils.Debug("GetDriverProfile - retrieved successfully",
		zap.String("driver_id", driver.ID.String()),
		zap.Bool("is_verified", driver.IsVerified),
		zap.Bool("is_online", driver.IsOnline))

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Profile retrieved", driver))
}

// UpdateDriverProfileRequest request body
type UpdateDriverProfileRequest struct {
	Languages []string `json:"languages,omitempty"`
}

// UpdateDriverProfile updates driver profile
func (c *DriverController) UpdateDriverProfile(ctx *gin.Context) {
	userID := ctx.GetString("userID")

	if userID == "" {
		utils.Warn("UpdateDriverProfile - empty user ID")
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	uid, err := uuid.Parse(userID)
	if err != nil {
		utils.Warn("UpdateDriverProfile - invalid user ID", zap.String("user_id", userID), zap.Error(err))
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	var req UpdateDriverProfileRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	updates := make(map[string]interface{})
	if len(req.Languages) > 0 {
		updates["languages"] = req.Languages
	}

	if len(updates) == 0 {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", "no fields to update"))
		return
	}

	driver, err := c.Service.UpdateDriverProfile(uid, updates)
	if err != nil {
		utils.Warn("UpdateDriverProfile - update failed", zap.String("user_id", uid.String()), zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Update failed", err.Error()))
		return
	}

	utils.Debug("UpdateDriverProfile - success", zap.String("driver_id", driver.ID.String()))
	ctx.JSON(http.StatusOK, utils.SuccessResponse("Profile updated", driver))
}

// GoOnlineRequest request body
type GoOnlineRequest struct {
	Lat float64 `json:"lat" binding:"required"`
	Lng float64 `json:"lng" binding:"required"`
}

// GoOnline marks driver as online
func (c *DriverController) GoOnline(ctx *gin.Context) {
	userID := ctx.GetString("userID")

	if userID == "" {
		utils.Warn("GoOnline - empty user ID")
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	uid, err := uuid.Parse(userID)
	if err != nil {
		utils.Warn("GoOnline - invalid user ID", zap.String("user_id", userID), zap.Error(err))
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	var req GoOnlineRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	if err := c.Service.GoOnline(uid, req.Lat, req.Lng); err != nil {
		utils.Warn("GoOnline - service error", zap.String("user_id", uid.String()), zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to go online", err.Error()))
		return
	}

	utils.Debug("GoOnline - success", zap.String("user_id", uid.String()))
	ctx.JSON(http.StatusOK, utils.SuccessResponse("You are now online", nil))
}

// GoOffline marks driver as offline
func (c *DriverController) GoOffline(ctx *gin.Context) {
	userID := ctx.GetString("userID")

	if userID == "" {
		utils.Warn("GoOffline - empty user ID")
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	uid, err := uuid.Parse(userID)
	if err != nil {
		utils.Warn("GoOffline - invalid user ID", zap.String("user_id", userID), zap.Error(err))
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	if err := c.Service.GoOffline(uid); err != nil {
		utils.Warn("GoOffline - service error", zap.String("user_id", uid.String()), zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to go offline", err.Error()))
		return
	}

	utils.Debug("GoOffline - success", zap.String("user_id", uid.String()))
	ctx.JSON(http.StatusOK, utils.SuccessResponse("You are now offline", nil))
}

// UpdateLocationRequest request body
type UpdateLocationRequest struct {
	Lat      float64 `json:"lat" binding:"required"`
	Lng      float64 `json:"lng" binding:"required"`
	Accuracy float64 `json:"accuracy,omitempty"`
}

// UpdateLocation updates driver location
func (c *DriverController) UpdateLocation(ctx *gin.Context) {
	userID := ctx.GetString("userID")

	if userID == "" {
		utils.Warn("UpdateLocation - empty user ID")
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	uid, err := uuid.Parse(userID)
	if err != nil {
		utils.Warn("UpdateLocation - invalid user ID", zap.String("user_id", userID), zap.Error(err))
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	var req UpdateLocationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	if err := c.Service.UpdateLocation(uid, req.Lat, req.Lng, req.Accuracy); err != nil {
		utils.Warn("UpdateLocation - service error", zap.String("user_id", uid.String()), zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Update failed", err.Error()))
		return
	}

	utils.Debug("UpdateLocation - success", zap.String("user_id", uid.String()))
	ctx.JSON(http.StatusOK, utils.SuccessResponse("Location updated", nil))
}

// GetDriverEarnings gets driver earnings
func (c *DriverController) GetDriverEarnings(ctx *gin.Context) {
	userID := ctx.GetString("userID")

	if userID == "" {
		utils.Warn("GetDriverEarnings - empty user ID")
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	uid, err := uuid.Parse(userID)
	if err != nil {
		utils.Warn("GetDriverEarnings - invalid user ID", zap.String("user_id", userID), zap.Error(err))
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	earnings, err := c.Service.GetDriverEarnings(uid)
	if err != nil {
		utils.Warn("GetDriverEarnings - not found", zap.String("user_id", uid.String()), zap.Error(err))
		ctx.JSON(http.StatusNotFound, utils.SanitizedErrorResponse("Earnings not found", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Earnings retrieved", earnings))
}

// GetDriverStats gets driver statistics
func (c *DriverController) GetDriverStats(ctx *gin.Context) {
	userID := ctx.GetString("userID")

	if userID == "" {
		utils.Warn("GetDriverStats - empty user ID")
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	uid, err := uuid.Parse(userID)
	if err != nil {
		utils.Warn("GetDriverStats - invalid user ID", zap.String("user_id", userID), zap.Error(err))
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	stats, err := c.Service.GetDriverStats(uid)
	if err != nil {
		utils.Warn("GetDriverStats - not found", zap.String("user_id", uid.String()), zap.Error(err))
		ctx.JSON(http.StatusNotFound, utils.SanitizedErrorResponse("Stats not found", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Stats retrieved", stats))
}

// GetPendingVerifications (Admin) gets pending driver verifications
func (c *DriverController) GetPendingVerifications(ctx *gin.Context) {
	page, err := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}
	perPage, err := strconv.Atoi(ctx.DefaultQuery("per_page", "10"))
	if err != nil || perPage < 1 || perPage > 100 {
		perPage = 10
	}

	drivers, total, err := c.Service.GetPendingVerifications(page, perPage)
	if err != nil {
		utils.Warn("GetPendingVerifications - service error", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to fetch pending verifications", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.PaginatedResponse(drivers, page, perPage, total))
}

// VerifyDriverRequest request body
type VerifyDriverRequest struct {
	DriverID string `json:"driver_id" binding:"required"`
}

// VerifyDriver (Admin) verifies a driver
func (c *DriverController) VerifyDriver(ctx *gin.Context) {
	adminID := ctx.GetString("userID")

	if adminID == "" {
		utils.Warn("VerifyDriver - empty admin ID")
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	aid, err := uuid.Parse(adminID)
	if err != nil {
		utils.Warn("VerifyDriver - invalid admin ID", zap.String("admin_id", adminID), zap.Error(err))
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	var req VerifyDriverRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	driverID, err := uuid.Parse(req.DriverID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", "invalid driver_id"))
		return
	}

	if err := c.Service.VerifyDriver(driverID, aid); err != nil {
		utils.Warn("VerifyDriver - verification failed", zap.String("driver_id", driverID.String()), zap.String("admin_id", aid.String()), zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Verification failed", err.Error()))
		return
	}

	utils.Debug("VerifyDriver - success", zap.String("driver_id", driverID.String()), zap.String("admin_id", aid.String()))
	ctx.JSON(http.StatusOK, utils.SuccessResponse("Driver verified successfully", nil))
}
