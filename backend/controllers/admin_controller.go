package controllers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"rapido-backend/config"
	"rapido-backend/database"
	"rapido-backend/models"
	"rapido-backend/services"
	"rapido-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AdminController struct {
	DB *gorm.DB
}

func NewAdminController() *AdminController {
	return &AdminController{DB: database.DB}
}

// GetDashboardStats gets dashboard statistics
func (c *AdminController) GetDashboardStats(ctx *gin.Context) {
	now := time.Now()
	today := now.Truncate(24 * time.Hour)
	thisWeek := now.AddDate(0, 0, -7)
	thisMonth := now.AddDate(0, -1, 0)

	var todayRides, weekRides, monthRides, totalRides int64
	c.DB.Model(&models.Ride{}).Where("DATE(created_at) = DATE(?)", today).Count(&todayRides)
	c.DB.Model(&models.Ride{}).Where("created_at >= ?", thisWeek).Count(&weekRides)
	c.DB.Model(&models.Ride{}).Where("created_at >= ?", thisMonth).Count(&monthRides)
	c.DB.Model(&models.Ride{}).Count(&totalRides)

	var todayRevenue, totalRevenue float64
	c.DB.Model(&models.Payment{}).Where("status = ? AND DATE(created_at) = DATE(?)", "completed", today).Select("COALESCE(SUM(amount), 0)").Scan(&todayRevenue)
	c.DB.Model(&models.Payment{}).Where("status = ?", "completed").Select("COALESCE(SUM(amount), 0)").Scan(&totalRevenue)

	var activeDrivers, totalDrivers, pendingVerifications int64
	c.DB.Model(&models.Driver{}).Where("is_online = ? AND is_verified = ?", true, true).Count(&activeDrivers)
	c.DB.Model(&models.Driver{}).Count(&totalDrivers)
	c.DB.Model(&models.Driver{}).Where("is_verified = ?", false).Count(&pendingVerifications)

	var totalUsers int64
	c.DB.Model(&models.User{}).Where("role = ?", "rider").Count(&totalUsers)

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Dashboard stats", map[string]interface{}{
		"rides": map[string]interface{}{
			"today":      todayRides,
			"this_week":  weekRides,
			"this_month": monthRides,
			"total":      totalRides,
		},
		"revenue": map[string]interface{}{
			"today": todayRevenue,
			"total": totalRevenue,
		},
		"drivers": map[string]interface{}{
			"active":                activeDrivers,
			"total":                 totalDrivers,
			"pending_verifications": pendingVerifications,
		},
		"users": map[string]interface{}{
			"total": totalUsers,
		},
	}))
}

// GetAllRides gets all rides with filters
func (c *AdminController) GetAllRides(ctx *gin.Context) {
	page, err := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}
	perPage, err := strconv.Atoi(ctx.DefaultQuery("per_page", "20"))
	if err != nil || perPage < 1 || perPage > 100 {
		perPage = 20
	}
	status := ctx.Query("status")

	offset := (page - 1) * perPage

	query := c.DB.Model(&models.Ride{}).Preload("Rider").Preload("Driver.User")
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var rides []models.Ride
	var count int64

	if err := query.Count(&count).Error; err != nil {
		utils.Warn("GetAllRides - count failed", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to fetch rides", err.Error()))
		return
	}

	if err := query.Order("created_at DESC").Offset(offset).Limit(perPage).Find(&rides).Error; err != nil {
		utils.Warn("GetAllRides - query failed", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to fetch rides", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.PaginatedResponse(rides, page, perPage, count))
}

// GetAllUsers gets all users
func (c *AdminController) GetAllUsers(ctx *gin.Context) {
	page, err := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}
	perPage, err := strconv.Atoi(ctx.DefaultQuery("per_page", "20"))
	if err != nil || perPage < 1 || perPage > 100 {
		perPage = 20
	}

	offset := (page - 1) * perPage

	var users []models.User
	var count int64

	if err := c.DB.Model(&models.User{}).Where("role = ?", "rider").Count(&count).Error; err != nil {
		utils.Warn("GetAllUsers - count failed", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to fetch users", err.Error()))
		return
	}
	if err := c.DB.Where("role = ?", "rider").Order("created_at DESC").Offset(offset).Limit(perPage).Find(&users).Error; err != nil {
		utils.Warn("GetAllUsers - query failed", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to fetch users", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.PaginatedResponse(users, page, perPage, count))
}

// GetAllDrivers gets all drivers
func (c *AdminController) GetAllDrivers(ctx *gin.Context) {
	page, err := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}
	perPage, err := strconv.Atoi(ctx.DefaultQuery("per_page", "20"))
	if err != nil || perPage < 1 || perPage > 100 {
		perPage = 20
	}
	verified := ctx.Query("verified")

	offset := (page - 1) * perPage

	query := c.DB.Model(&models.Driver{}).Preload("User")
	if verified != "" {
		query = query.Where("is_verified = ?", verified == "true")
	}

	var drivers []models.Driver
	var count int64

	if err := query.Count(&count).Error; err != nil {
		utils.Warn("GetAllDrivers - count failed", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to fetch drivers", err.Error()))
		return
	}
	if err := query.Order("created_at DESC").Offset(offset).Limit(perPage).Find(&drivers).Error; err != nil {
		utils.Warn("GetAllDrivers - query failed", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to fetch drivers", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.PaginatedResponse(drivers, page, perPage, count))
}

// GetAllPayments gets all payments
func (c *AdminController) GetAllPayments(ctx *gin.Context) {
	page, err := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}
	perPage, err := strconv.Atoi(ctx.DefaultQuery("per_page", "20"))
	if err != nil || perPage < 1 || perPage > 100 {
		perPage = 20
	}

	offset := (page - 1) * perPage

	var payments []models.Payment
	var count int64

	if err := c.DB.Model(&models.Payment{}).Count(&count).Error; err != nil {
		utils.Warn("GetAllPayments - count failed", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to fetch payments", err.Error()))
		return
	}
	if err := c.DB.Order("created_at DESC").Offset(offset).Limit(perPage).Find(&payments).Error; err != nil {
		utils.Warn("GetAllPayments - query failed", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to fetch payments", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.PaginatedResponse(payments, page, perPage, count))
}

// GetPendingWithdrawals gets pending withdrawal requests
func (c *AdminController) GetPendingWithdrawals(ctx *gin.Context) {
	page, err := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}
	perPage, err := strconv.Atoi(ctx.DefaultQuery("per_page", "20"))
	if err != nil || perPage < 1 || perPage > 100 {
		perPage = 20
	}

	offset := (page - 1) * perPage

	var withdrawals []models.Withdrawal
	var count int64

	if err := c.DB.Model(&models.Withdrawal{}).Where("status = ?", "pending").Count(&count).Error; err != nil {
		utils.Warn("GetPendingWithdrawals - count failed", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to fetch withdrawals", err.Error()))
		return
	}
	if err := c.DB.Where("status = ?", "pending").Preload("Driver.User").Order("created_at ASC").Offset(offset).Limit(perPage).Find(&withdrawals).Error; err != nil {
		utils.Warn("GetPendingWithdrawals - query failed", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to fetch withdrawals", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.PaginatedResponse(withdrawals, page, perPage, count))
}

// ProcessWithdrawalRequest request body
type ProcessWithdrawalRequest struct {
	WithdrawalID    string `json:"withdrawal_id" binding:"required"`
	Approved        bool   `json:"approved"`
	RejectionReason string `json:"rejection_reason,omitempty"`
}

// ProcessWithdrawal processes a withdrawal
func (c *AdminController) ProcessWithdrawal(ctx *gin.Context) {
	adminID := ctx.GetString("userID")
	if adminID == "" {
		utils.Warn("ProcessWithdrawal - empty admin ID")
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	aid, err := uuid.Parse(adminID)
	if err != nil {
		utils.Warn("ProcessWithdrawal - invalid admin ID", zap.String("admin_id", adminID), zap.Error(err))
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	var req ProcessWithdrawalRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	withdrawalID, err := uuid.Parse(req.WithdrawalID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", "invalid withdrawal_id"))
		return
	}

	service := services.NewPaymentService()
	if err := service.ProcessWithdrawal(withdrawalID, aid, req.Approved, req.RejectionReason); err != nil {
		utils.Warn("ProcessWithdrawal - service error", zap.String("withdrawal_id", withdrawalID.String()), zap.String("admin_id", aid.String()), zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to process withdrawal", err.Error()))
		return
	}

	utils.Debug("ProcessWithdrawal - success", zap.String("withdrawal_id", withdrawalID.String()), zap.String("admin_id", aid.String()))
	ctx.JSON(http.StatusOK, utils.SuccessResponse("Withdrawal processed", nil))
}

// CreateSurgePricingRequest request body
type CreateSurgePricingRequest struct {
	AreaName      string  `json:"area_name" binding:"required"`
	Latitude      float64 `json:"lat" binding:"required"`
	Longitude     float64 `json:"lng" binding:"required"`
	RadiusKM      float64 `json:"radius_km" binding:"required"`
	Multiplier    float64 `json:"multiplier" binding:"required"`
	Reason        string  `json:"reason,omitempty"`
	DurationHours int     `json:"duration_hours"`
}

// CreateSurgePricing creates surge pricing
func (c *AdminController) CreateSurgePricing(ctx *gin.Context) {
	adminID := ctx.GetString("userID")
	if adminID == "" {
		utils.Warn("CreateSurgePricing - empty admin ID")
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	aid, err := uuid.Parse(adminID)
	if err != nil {
		utils.Warn("CreateSurgePricing - invalid admin ID", zap.String("admin_id", adminID), zap.Error(err))
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	var req CreateSurgePricingRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	now := time.Now()
	endTime := now.Add(time.Duration(req.DurationHours) * time.Hour)

	surge := &models.SurgePricing{
		AreaName:   req.AreaName,
		Latitude:   req.Latitude,
		Longitude:  req.Longitude,
		RadiusKM:   req.RadiusKM,
		Multiplier: req.Multiplier,
		IsActive:   true,
		Reason:     req.Reason,
		StartTime:  &now,
		EndTime:    &endTime,
		CreatedBy:  aid,
	}

	if err := c.DB.Create(surge).Error; err != nil {
		utils.Warn("CreateSurgePricing - db error", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to create surge pricing", err.Error()))
		return
	}

	utils.Debug("CreateSurgePricing - created", zap.String("surge_id", surge.ID.String()), zap.String("admin_id", aid.String()))
	ctx.JSON(http.StatusCreated, utils.SuccessResponse("Surge pricing created", surge))
}

// RemoveSurgePricing removes surge pricing
func (c *AdminController) RemoveSurgePricing(ctx *gin.Context) {
	surgeID := ctx.Param("id")
	sid, err := uuid.Parse(surgeID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid surge ID", ""))
		return
	}

	if err := c.DB.Model(&models.SurgePricing{}).Where("id = ?", sid).Update("is_active", false).Error; err != nil {
		utils.Warn("RemoveSurgePricing - db error", zap.String("surge_id", sid.String()), zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to remove surge pricing", err.Error()))
		return
	}

	utils.Debug("RemoveSurgePricing - success", zap.String("surge_id", sid.String()))
	ctx.JSON(http.StatusOK, utils.SuccessResponse("Surge pricing removed", nil))
}

// CreatePromoCodeRequest request body
type CreatePromoCodeRequest struct {
	Code           string   `json:"code" binding:"required"`
	Description    string   `json:"description"`
	DiscountType   string   `json:"discount_type" binding:"required,oneof=percentage fixed"`
	DiscountValue  float64  `json:"discount_value" binding:"required"`
	MaxDiscount    float64  `json:"max_discount"`
	MinRideAmount  float64  `json:"min_ride_amount"`
	MaxUses        int      `json:"max_uses"`
	MaxUsesPerUser int      `json:"max_uses_per_user"`
	VehicleTypes   []string `json:"vehicle_types,omitempty"`
	StartDate      string   `json:"start_date,omitempty"`
	EndDate        string   `json:"end_date,omitempty"`
}

// CreatePromoCode creates a promo code
func (c *AdminController) CreatePromoCode(ctx *gin.Context) {
	adminID := ctx.GetString("userID")
	if adminID == "" {
		utils.Warn("CreatePromoCode - empty admin ID")
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	aid, err := uuid.Parse(adminID)
	if err != nil {
		utils.Warn("CreatePromoCode - invalid admin ID", zap.String("admin_id", adminID), zap.Error(err))
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	var req CreatePromoCodeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	promo := &models.PromoCode{
		Code:                   req.Code,
		Description:            req.Description,
		DiscountType:           req.DiscountType,
		DiscountValue:          req.DiscountValue,
		MaxDiscount:            req.MaxDiscount,
		MinRideAmount:          req.MinRideAmount,
		MaxUses:                req.MaxUses,
		MaxUsesPerUser:         req.MaxUsesPerUser,
		ApplicableVehicleTypes: req.VehicleTypes,
		IsActive:               true,
		CreatedBy:              aid,
	}

	if req.StartDate != "" {
		if start, err := time.Parse(time.RFC3339, req.StartDate); err == nil {
			promo.StartDate = &start
		}
	}
	if req.EndDate != "" {
		if end, err := time.Parse(time.RFC3339, req.EndDate); err == nil {
			promo.EndDate = &end
		}
	}

	if err := c.DB.Create(promo).Error; err != nil {
		utils.Warn("CreatePromoCode - db error", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to create promo code", err.Error()))
		return
	}

	utils.Debug("CreatePromoCode - created", zap.String("promo_code", promo.Code), zap.String("admin_id", aid.String()))
	ctx.JSON(http.StatusCreated, utils.SuccessResponse("Promo code created", promo))
}

// GetReports gets various reports
func (c *AdminController) GetReports(ctx *gin.Context) {
	reportType := ctx.Query("type")

	switch reportType {
	case "daily_earnings":
		c.getDailyEarningsReport(ctx)
	case "driver_performance":
		c.getDriverPerformanceReport(ctx)
	case "ride_funnel":
		c.getRideFunnelReport(ctx)
	case "peak_hours":
		c.getPeakHoursReport(ctx)
	case "revenue_summary":
		c.getRevenueSummaryReport(ctx)
	default:
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid report type", ""))
	}
}

func (c *AdminController) getDailyEarningsReport(ctx *gin.Context) {
	days, err := strconv.Atoi(ctx.DefaultQuery("days", "7"))
	if err != nil || days <= 0 || days > 90 {
		days = 7
	}

	type row struct {
		Date         time.Time `json:"date"`
		GrossRevenue float64   `json:"gross_revenue"`
		TotalRides   int64     `json:"total_rides"`
	}

	var rows []row
	since := time.Now().AddDate(0, 0, -days)

	err = c.DB.Raw(`
		SELECT DATE(created_at) AS date,
			COALESCE(SUM(amount), 0) AS gross_revenue,
			COUNT(*) AS total_rides
		FROM payments
		WHERE status = ? AND created_at >= ?
		GROUP BY DATE(created_at)
		ORDER BY DATE(created_at) ASC
	`, models.PaymentStatusCompleted, since).Scan(&rows).Error
	if err != nil {
		utils.Warn("getDailyEarningsReport - db error", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to build daily earnings report", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Daily earnings report", map[string]interface{}{
		"window_days": days,
		"items":       rows,
	}))
}

func (c *AdminController) getDriverPerformanceReport(ctx *gin.Context) {
	limit, err := strconv.Atoi(ctx.DefaultQuery("limit", "20"))
	if err != nil || limit <= 0 || limit > 100 {
		limit = 20
	}

	type driverPerf struct {
		DriverID         uuid.UUID `json:"driver_id"`
		Name             string    `json:"name"`
		Rating           float64   `json:"rating"`
		AcceptanceScore  float64   `json:"acceptance_score"`
		CancellationRate float64   `json:"cancellation_rate"`
		CompletedRides   int64     `json:"completed_rides"`
		CancelledRides   int64     `json:"cancelled_rides"`
		TotalEarnings    float64   `json:"total_earnings"`
		CurrentBalance   float64   `json:"current_balance"`
	}

	var items []driverPerf
	err = c.DB.Raw(`
		SELECT d.id AS driver_id,
			COALESCE(u.name, '') AS name,
			d.rating,
			d.acceptance_score,
			d.cancellation_rate,
			COALESCE(SUM(CASE WHEN r.status = ? THEN 1 ELSE 0 END), 0) AS completed_rides,
			COALESCE(SUM(CASE WHEN r.status = ? THEN 1 ELSE 0 END), 0) AS cancelled_rides,
			COALESCE(de.total_earnings, 0) AS total_earnings,
			COALESCE(de.current_balance, 0) AS current_balance
		FROM drivers d
		LEFT JOIN users u ON u.id = d.user_id
		LEFT JOIN rides r ON r.driver_id = d.id
		LEFT JOIN driver_earnings de ON de.driver_id = d.id
		GROUP BY d.id, u.name, d.rating, d.acceptance_score, d.cancellation_rate, de.total_earnings, de.current_balance
		ORDER BY completed_rides DESC, d.rating DESC
		LIMIT ?
	`, models.RideStatusCompleted, models.RideStatusCancelled, limit).Scan(&items).Error
	if err != nil {
		utils.Warn("getDriverPerformanceReport - db error", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to build driver performance report", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Driver performance report", map[string]interface{}{
		"limit": limit,
		"items": items,
	}))
}

func (c *AdminController) getRideFunnelReport(ctx *gin.Context) {
	type funnelRow struct {
		Status string `json:"status"`
		Count  int64  `json:"count"`
	}

	var rows []funnelRow
	if err := c.DB.Model(&models.Ride{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Scan(&rows).Error; err != nil {
		utils.Warn("getRideFunnelReport - db error", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to build ride funnel report", err.Error()))
		return
	}

	total := int64(0)
	for _, row := range rows {
		total += row.Count
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Ride funnel report", map[string]interface{}{
		"total": total,
		"items": rows,
	}))
}

func (c *AdminController) getPeakHoursReport(ctx *gin.Context) {
	days, err := strconv.Atoi(ctx.DefaultQuery("days", "7"))
	if err != nil || days <= 0 || days > 90 {
		days = 7
	}

	type hourRow struct {
		Hour      int   `json:"hour"`
		RideCount int64 `json:"ride_count"`
	}

	var rows []hourRow
	since := time.Now().AddDate(0, 0, -days)
	err = c.DB.Raw(`
		SELECT EXTRACT(HOUR FROM created_at)::int AS hour,
			COUNT(*) AS ride_count
		FROM rides
		WHERE created_at >= ?
		GROUP BY EXTRACT(HOUR FROM created_at)
		ORDER BY ride_count DESC
	`, since).Scan(&rows).Error
	if err != nil {
		utils.Warn("getPeakHoursReport - db error", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to build peak hours report", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Peak hours report", map[string]interface{}{
		"window_days": days,
		"items":       rows,
	}))
}

func (c *AdminController) getRevenueSummaryReport(ctx *gin.Context) {
	start := time.Now().AddDate(0, -1, 0)
	end := time.Now()

	if startParam := ctx.Query("start"); startParam != "" {
		if t, err := time.Parse(time.RFC3339, startParam); err == nil {
			start = t
		}
	}
	if endParam := ctx.Query("end"); endParam != "" {
		if t, err := time.Parse(time.RFC3339, endParam); err == nil {
			end = t
		}
	}

	var grossRevenue float64
	var totalPayments int64
	var refundedAmount float64

	if err := c.DB.Model(&models.Payment{}).
		Where("status = ? AND created_at BETWEEN ? AND ?", models.PaymentStatusCompleted, start, end).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&grossRevenue).Error; err != nil {
		utils.Warn("getRevenueSummaryReport - grossRevenue query failed", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to build revenue summary", err.Error()))
		return
	}

	if err := c.DB.Model(&models.Payment{}).
		Where("created_at BETWEEN ? AND ?", start, end).
		Count(&totalPayments).Error; err != nil {
		utils.Warn("getRevenueSummaryReport - totalPayments query failed", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to build revenue summary", err.Error()))
		return
	}

	if err := c.DB.Model(&models.Payment{}).
		Where("created_at BETWEEN ? AND ?", start, end).
		Select("COALESCE(SUM(refund_amount), 0)").
		Scan(&refundedAmount).Error; err != nil {
		utils.Warn("getRevenueSummaryReport - refundedAmount query failed", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to build revenue summary", err.Error()))
		return
	}

	netRevenue := grossRevenue - refundedAmount
	if netRevenue < 0 {
		netRevenue = 0
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Revenue summary report", map[string]interface{}{
		"start":           start,
		"end":             end,
		"gross_revenue":   grossRevenue,
		"refunded_amount": refundedAmount,
		"net_revenue":     netRevenue,
		"total_payments":  totalPayments,
	}))
}

// GetLedgerAccounts gets all ledger accounts with filters
func (c *AdminController) GetLedgerAccounts(ctx *gin.Context) {
	page, err := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}
	perPage, err := strconv.Atoi(ctx.DefaultQuery("per_page", "20"))
	if err != nil || perPage < 1 || perPage > 100 {
		perPage = 20
	}
	accountType := ctx.Query("account_type")
	ownerID := ctx.Query("owner_id")

	offset := (page - 1) * perPage

	query := c.DB.Model(&models.LedgerAccount{})
	if accountType != "" {
		query = query.Where("account_type = ?", accountType)
	}
	if ownerID != "" {
		query = query.Where("owner_id = ?", ownerID)
	}

	var accounts []models.LedgerAccount
	var count int64

	if err := query.Count(&count).Error; err != nil {
		utils.Warn("GetLedgerAccounts - count failed", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to fetch ledger accounts", err.Error()))
		return
	}
	if err := query.Order("created_at DESC").Offset(offset).Limit(perPage).Find(&accounts).Error; err != nil {
		utils.Warn("GetLedgerAccounts - query failed", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to fetch ledger accounts", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.PaginatedResponse(accounts, page, perPage, count))
}

// GetLedgerEntries gets ledger entries for an account
func (c *AdminController) GetLedgerEntries(ctx *gin.Context) {
	accountID := ctx.Query("account_id")
	referenceID := ctx.Query("reference_id")
	batchID := ctx.Query("batch_id")
	page, err := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}
	perPage, err := strconv.Atoi(ctx.DefaultQuery("per_page", "50"))
	if err != nil || perPage < 1 || perPage > 500 {
		perPage = 50
	}

	offset := (page - 1) * perPage

	query := c.DB.Model(&models.LedgerEntry{})
	if accountID != "" {
		query = query.Where("account_id = ?", accountID)
	}
	if referenceID != "" {
		query = query.Where("reference_id = ?", referenceID)
	}
	if batchID != "" {
		query = query.Where("batch_id = ?", batchID)
	}

	var entries []models.LedgerEntry
	var count int64

	if err := query.Count(&count).Error; err != nil {
		utils.Warn("GetLedgerEntries - count failed", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to fetch ledger entries", err.Error()))
		return
	}
	if err := query.Order("created_at DESC").Offset(offset).Limit(perPage).Find(&entries).Error; err != nil {
		utils.Warn("GetLedgerEntries - query failed", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to fetch ledger entries", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.PaginatedResponse(entries, page, perPage, count))
}

// AuditLedgerBatchRequest request body
type AuditLedgerBatchRequest struct {
	BatchID string `json:"batch_id" binding:"required"`
}

// AuditLedgerBatch audits all entries in a batch
func (c *AdminController) AuditLedgerBatch(ctx *gin.Context) {
	var req AuditLedgerBatchRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	batchID, err := uuid.Parse(req.BatchID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid batch ID", ""))
		return
	}

	var entries []models.LedgerEntry
	if err := c.DB.Where("batch_id = ?", batchID).Order("created_at ASC").Find(&entries).Error; err != nil {
		utils.Warn("AuditLedgerBatch - db error", zap.String("batch_id", batchID.String()), zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to fetch batch entries", err.Error()))
		return
	}

	// Calculate batch audit summary
	audit := map[string]interface{}{
		"batch_id":      batchID,
		"entry_count":   len(entries),
		"total_debits":  0.0,
		"total_credits": 0.0,
		"entries":       entries,
		"balanced":      true,
	}

	if len(entries) > 0 {
		totalDebits := 0.0
		totalCredits := 0.0

		for _, entry := range entries {
			if entry.Direction == models.LedgerDirectionDebit {
				totalDebits += entry.Amount
			} else {
				totalCredits += entry.Amount
			}
		}

		audit["total_debits"] = totalDebits
		audit["total_credits"] = totalCredits
		// Allow small floating-point discrepancy (0.01 paise)
		audit["balanced"] = (totalDebits-totalCredits) < 0.01 && (totalDebits-totalCredits) > -0.01
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Batch audit", audit))
}

// GetAccountBalance gets balance for a specific account
func (c *AdminController) GetAccountBalance(ctx *gin.Context) {
	accountID := ctx.Query("account_id")

	if accountID == "" {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("account_id is required", ""))
		return
	}

	accID, err := uuid.Parse(accountID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid account ID", ""))
		return
	}

	var account models.LedgerAccount
	if err := c.DB.Where("id = ?", accID).First(&account).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusNotFound, utils.ErrorResponse("Account not found", ""))
		} else {
			utils.Warn("GetAccountBalance - db error", zap.String("account_id", accID.String()), zap.Error(err))
			ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to fetch account", err.Error()))
		}
		return
	}

	// Get entry count for this account
	var entryCount int64
	c.DB.Model(&models.LedgerEntry{}).Where("account_id = ?", accID).Count(&entryCount)

	// Get latest 5 entries
	var latestEntries []models.LedgerEntry
	c.DB.Where("account_id = ?", accID).Order("created_at DESC").Limit(5).Find(&latestEntries)

	response := map[string]interface{}{
		"account":        account,
		"entry_count":    entryCount,
		"latest_entries": latestEntries,
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Account balance", response))
}

// GetAppConfig returns full app configuration (admin only)
func (c *AdminController) GetAppConfig(ctx *gin.Context) {
	configService := services.NewConfigService()
	config := configService.GetAdminConfig()
	ctx.JSON(http.StatusOK, utils.SuccessResponse("App config retrieved", config))
}

// UpdateAppConfig updates app configuration (admin only)
func (c *AdminController) UpdateAppConfig(ctx *gin.Context) {
	var req struct {
		Key   string      `json:"key" binding:"required"`
		Value interface{} `json:"value" binding:"required"`
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	configService := services.NewConfigService()
	if err := configService.UpdateConfig(req.Key, req.Value); err != nil {
		utils.Warn("UpdateAppConfig - service error", zap.String("key", req.Key), zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to update config", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Config updated", map[string]interface{}{
		"key":   req.Key,
		"value": req.Value,
	}))
}

// ListPendingDrivers returns drivers awaiting verification
type DriverVerificationStatus struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	Email         string    `json:"email"`
	Phone         string    `json:"phone"`
	LicenseNumber string    `json:"license_number"`
	RCNumber      string    `json:"rc_number"`
	VehicleType   string    `json:"vehicle_type"`
	VehicleMake   string    `json:"vehicle_make"`
	VehicleModel  string    `json:"vehicle_model"`
	CreatedAt     time.Time `json:"created_at"`
}

func (c *AdminController) ListPendingDrivers(ctx *gin.Context) {
	page, err := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}
	perPage, err := strconv.Atoi(ctx.DefaultQuery("per_page", "10"))
	if err != nil || perPage < 1 || perPage > 100 {
		perPage = 10
	}

	var drivers []models.Driver
	var total int64

	if err := c.DB.Model(&models.Driver{}).Where("is_verified = ?", false).Count(&total).Error; err != nil {
		utils.Warn("ListPendingDrivers - count failed", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to fetch drivers", err.Error()))
		return
	}

	offset := (page - 1) * perPage
	if err := c.DB.Where("is_verified = ?", false).
		Order("created_at DESC").
		Offset(offset).Limit(perPage).
		Find(&drivers).Error; err != nil {
		utils.Warn("ListPendingDrivers - query failed", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to fetch drivers", err.Error()))
		return
	}

	// Enrich with user data
	var results []DriverVerificationStatus
	for _, driver := range drivers {
		var user models.User
		c.DB.Where("id = ?", driver.UserID).First(&user)

		results = append(results, DriverVerificationStatus{
			ID:            driver.ID,
			Name:          user.Name,
			Email:         user.Email,
			Phone:         user.Phone,
			LicenseNumber: driver.LicenseNumber,
			RCNumber:      driver.RCNumber,
			CreatedAt:     driver.CreatedAt,
		})
	}

	ctx.JSON(http.StatusOK, utils.PaginatedResponse(results, page, perPage, total))
}

// AdminVerifyDriverRequest request body
type AdminVerifyDriverRequest struct {
	// Prefer `verified` boolean, but accept `action` for backwards-compatibility
	Verified *bool  `json:"verified,omitempty"`
	Action   string `json:"action,omitempty"` // "approve" | "reject"
	Notes    string `json:"notes,omitempty"`
}

// VerifyDriver approves or rejects a driver
func (c *AdminController) VerifyDriver(ctx *gin.Context) {
	driverID := ctx.Param("id")
	uid, err := uuid.Parse(driverID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid driver ID", ""))
		return
	}

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

	var req AdminVerifyDriverRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	// Determine final verified boolean from either `verified` or `action`
	var verified bool
	if req.Verified != nil {
		verified = *req.Verified
	} else if req.Action != "" {
		switch strings.ToLower(req.Action) {
		case "approve", "approved":
			verified = true
		case "reject", "rejected":
			verified = false
		default:
			ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid action", "action must be 'approve' or 'reject'"))
			return
		}
	} else {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", "either 'verified' (bool) or 'action' (approve|reject) is required"))
		return
	}

	var driver models.Driver
	if err := c.DB.Where("id = ?", uid).First(&driver).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusNotFound, utils.ErrorResponse("Driver not found", ""))
		} else {
			utils.Warn("VerifyDriver - db error", zap.String("driver_id", uid.String()), zap.Error(err))
			ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to fetch driver", err.Error()))
		}
		return
	}

	// Update verification status
	updates := map[string]interface{}{
		"is_verified": verified,
		"updated_at":  time.Now(),
	}

	if err := c.DB.Model(&driver).Updates(updates).Error; err != nil {
		utils.Warn("VerifyDriver - update failed", zap.String("driver_id", uid.String()), zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to update driver", err.Error()))
		return
	}

	// Log the verification action
	status := "approved"
	if !verified {
		status = "rejected"
	}
	c.DB.Create(&models.AdminActivityLog{
		AdminID:     aid,
		Action:      "verify_driver",
		EntityType:  "driver",
		EntityID:    &driver.ID,
		Description: "Driver " + driver.ID.String() + " " + status + ". Notes: " + req.Notes,
	})

	message := "Driver approved successfully"
	if !verified {
		message = "Driver rejected"
	}

	utils.Debug("VerifyDriver - success", zap.String("driver_id", driver.ID.String()), zap.String("admin_id", aid.String()), zap.Bool("verified", verified))
	ctx.JSON(http.StatusOK, utils.SuccessResponse(message, map[string]interface{}{
		"driver_id":   driver.ID,
		"verified":    verified,
		"is_verified": verified,
	}))
}

// GetDriverDetails returns detailed driver info for admin review
func (c *AdminController) GetDriverDetails(ctx *gin.Context) {
	driverID := ctx.Param("id")
	uid, err := uuid.Parse(driverID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid driver ID", ""))
		return
	}

	var driver models.Driver
	if err := c.DB.Where("id = ?", uid).First(&driver).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusNotFound, utils.ErrorResponse("Driver not found", ""))
		} else {
			utils.Warn("GetDriverDetails - db error", zap.String("driver_id", uid.String()), zap.Error(err))
			ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to fetch driver", err.Error()))
		}
		return
	}

	var user models.User
	c.DB.Where("id = ?", driver.UserID).First(&user)

	var vehicle models.Vehicle
	c.DB.Where("driver_id = ?", driver.ID).First(&vehicle)

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Driver details", map[string]interface{}{
		"driver": driver,
		"user": map[string]interface{}{
			"id":    user.ID,
			"name":  user.Name,
			"email": user.Email,
			"phone": user.Phone,
		},
		"vehicle": vehicle,
	}))
}

// CreateDriverRequest request for admin to manually create a driver
type CreateDriverRequest struct {
	// User info
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Phone    string `json:"phone" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`

	// Driver info
	LicenseNumber string   `json:"license_number" binding:"required"`
	LicenseImage  string   `json:"license_image" binding:"required"`
	LicenseExpiry string   `json:"license_expiry" binding:"required"`
	RCNumber      string   `json:"rc_number" binding:"required"`
	RCImage       string   `json:"rc_image" binding:"required"`
	AadhaarNumber string   `json:"aadhaar_number" binding:"required"`
	AadhaarImage  string   `json:"aadhaar_image" binding:"required"`
	Languages     []string `json:"languages"`

	// Vehicle info
	VehicleType        string `json:"vehicle_type" binding:"required"`
	VehicleMake        string `json:"vehicle_make" binding:"required"`
	VehicleModel       string `json:"vehicle_model" binding:"required"`
	VehicleYear        int    `json:"vehicle_year" binding:"required"`
	VehicleColor       string `json:"vehicle_color" binding:"required"`
	VehicleNumberPlate string `json:"vehicle_number_plate" binding:"required"`
	FuelType           string `json:"fuel_type" binding:"required"`
	VehicleImage       string `json:"vehicle_image"`

	// Auto-verify the driver immediately
	AutoVerify bool `json:"auto_verify"`
}

// CreateDriver allows admin to manually create a new driver user
func (c *AdminController) CreateDriver(ctx *gin.Context) {
	adminID := ctx.GetString("userID")
	if adminID == "" {
		utils.Warn("CreateDriver - empty admin ID")
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	aid, err := uuid.Parse(adminID)
	if err != nil {
		utils.Warn("CreateDriver - invalid admin ID", zap.String("admin_id", adminID), zap.Error(err))
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", ""))
		return
	}

	var req CreateDriverRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	// Check if email already exists
	var existingUser models.User
	if err := c.DB.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		ctx.JSON(http.StatusConflict, utils.ErrorResponse("Email already registered", ""))
		return
	}

	// Check if phone already exists
	if err := c.DB.Where("phone = ?", req.Phone).First(&existingUser).Error; err == nil {
		ctx.JSON(http.StatusConflict, utils.ErrorResponse("Phone already registered", ""))
		return
	}

	// Hash password
	hashedPassword, err := hashPassword(req.Password)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to hash password", err.Error()))
		return
	}

	// Create user
	user := models.User{
		Name:         req.Name,
		Email:        req.Email,
		Phone:        req.Phone,
		Role:         "driver",
		PasswordHash: hashedPassword,
		Provider:     "local",
	}
	if err := c.DB.Create(&user).Error; err != nil {
		utils.Warn("CreateDriver - create user failed", zap.String("email", req.Email), zap.String("phone", req.Phone), zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to create user", err.Error()))
		return
	}

	// Parse license expiry
	licenseExpiry, _ := time.Parse(time.RFC3339, req.LicenseExpiry)

	// Create driver
	driver := models.Driver{
		UserID:        user.ID,
		LicenseNumber: req.LicenseNumber,
		LicenseImage:  req.LicenseImage,
		LicenseExpiry: &licenseExpiry,
		RCNumber:      req.RCNumber,
		RCImage:       req.RCImage,
		AadhaarNumber: req.AadhaarNumber,
		AadhaarImage:  req.AadhaarImage,
		IsVerified:    req.AutoVerify,
		IsOnline:      false,
		IsActive:      true,
		Languages:     req.Languages,
	}
	if err := c.DB.Create(&driver).Error; err != nil {
		utils.Warn("CreateDriver - create driver failed", zap.String("user_id", user.ID.String()), zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to create driver", err.Error()))
		return
	}

	// Create vehicle
	vehicle := models.Vehicle{
		DriverID:     driver.ID,
		Type:         req.VehicleType,
		Make:         req.VehicleMake,
		Model:        req.VehicleModel,
		Year:         req.VehicleYear,
		Color:        req.VehicleColor,
		NumberPlate:  req.VehicleNumberPlate,
		FuelType:     req.FuelType,
		VehicleImage: req.VehicleImage,
	}
	if err := c.DB.Create(&vehicle).Error; err != nil {
		utils.Warn("CreateDriver - create vehicle failed", zap.String("driver_id", driver.ID.String()), zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to create vehicle", err.Error()))
		return
	}

	// Create driver earnings record
	earnings := models.DriverEarnings{
		DriverID: driver.ID,
	}
	c.DB.Create(&earnings)

	// Create rating summary
	ratingSummary := models.DriverRatingSummary{
		DriverID: driver.ID,
	}
	c.DB.Create(&ratingSummary)

	// Log the action
	c.DB.Create(&models.AdminActivityLog{
		AdminID:     aid,
		Action:      "create_driver",
		EntityType:  "driver",
		EntityID:    &driver.ID,
		Description: "Admin manually created driver: " + user.Name,
		NewValues:   models.JSONMap{"email": req.Email, "phone": req.Phone, "auto_verified": req.AutoVerify},
	})

	ctx.JSON(http.StatusCreated, utils.SuccessResponse("Driver created successfully", map[string]interface{}{
		"user": map[string]interface{}{
			"id":    user.ID,
			"name":  user.Name,
			"email": user.Email,
			"phone": user.Phone,
			"role":  user.Role,
		},
		"driver": map[string]interface{}{
			"id":          driver.ID,
			"is_verified": driver.IsVerified,
			"status":      "pending_verification",
		},
		"message":        "Driver can now login with email/phone and password",
		"login_endpoint": "/api/v1/auth/login",
	}))
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// DebugUserPassword checks if a user has password set (admin only)
func (c *AdminController) DebugUserPassword(ctx *gin.Context) {
	email := ctx.Query("email")
	if email == "" {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Email required", ""))
		return
	}

	var user models.User
	if err := c.DB.Where("email = ?", email).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusNotFound, utils.ErrorResponse("User not found", ""))
			return
		}
		utils.Warn("DebugUserPassword - db error", zap.String("email", email), zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to fetch user", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("User password status", map[string]interface{}{
		"user_id":      user.ID,
		"email":        user.Email,
		"phone":        user.Phone,
		"role":         user.Role,
		"has_password": user.PasswordHash != "",
		"password_hash_prefix": func() string {
			if len(user.PasswordHash) > 10 {
				return user.PasswordHash[:10] + "..."
			}
			return "empty"
		}(),
	}))
}

// ResetAdminPassword force updates admin password from ENV (emergency use)
func (c *AdminController) ResetAdminPassword(ctx *gin.Context) {
	cfg := config.Get()

	if cfg.Admin.Email == "" || cfg.Admin.Password == "" {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Admin credentials not configured in ENV", ""))
		return
	}

	var user models.User
	if err := c.DB.Where("email = ?", cfg.Admin.Email).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusNotFound, utils.ErrorResponse("Admin user not found", ""))
			return
		}
		utils.Warn("ResetAdminPassword - db error", zap.String("admin_email", cfg.Admin.Email), zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to fetch admin user", err.Error()))
		return
	}

	hashedPassword, err := hashPassword(cfg.Admin.Password)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to hash password", err.Error()))
		return
	}

	if err := c.DB.Model(&user).Update("password_hash", hashedPassword).Error; err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to update password", err.Error()))
		return
	}

	// Log the action
	adminID := ctx.GetString("userID")
	if adminID != "" {
		if aid, err := uuid.Parse(adminID); err == nil {
			c.DB.Create(&models.AdminActivityLog{
				AdminID:     aid,
				Action:      "reset_admin_password",
				EntityType:  "admin",
				EntityID:    &user.ID,
				Description: "Admin password reset from ENV",
			})
		}
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Admin password reset successfully", map[string]interface{}{
		"email":   cfg.Admin.Email,
		"message": "Admin can now login with ENV password",
	}))
}
