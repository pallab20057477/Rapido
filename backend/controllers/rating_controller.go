package controllers

import (
	"net/http"
	"strconv"

	"rapido-backend/models"
	"rapido-backend/services"
	"rapido-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RatingController handles rating-related requests
type RatingController struct {
	service *services.RatingService
}

// NewRatingController creates new controller
func NewRatingController() *RatingController {
	return &RatingController{
		service: services.NewRatingService(),
	}
}

// SubmitRatingRequest represents rating submission request
type SubmitRatingRequest struct {
	Rating     int    `json:"rating" binding:"required,min=1,max=5"`
	Review     string `json:"review"`
	Categories struct {
		Cleanliness  int `json:"cleanliness"`
		Punctuality  int `json:"punctuality"`
		DrivingSkill int `json:"driving_skill"`
		Behavior     int `json:"behavior"`
	} `json:"categories"`
}

// SubmitRating submits a rating for a completed ride
// POST /api/v1/rides/:id/rate
func (c *RatingController) SubmitRating(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	uid, _ := uuid.Parse(userID)
	userType := ctx.GetString("userType") // "rider" or "driver"

	rideID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid ride ID", err.Error()))
		return
	}

	var req SubmitRatingRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	// Convert categories
	categories := models.RatingCategories{
		Cleanliness:  req.Categories.Cleanliness,
		Punctuality:  req.Categories.Punctuality,
		DrivingSkill: req.Categories.DrivingSkill,
		Behavior:     req.Categories.Behavior,
	}

	rating, err := c.service.SubmitRating(rideID, uid, userType, req.Rating, req.Review, categories)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Failed to submit rating", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Rating submitted successfully", rating))
}

// GetDriverReviews gets all reviews for a driver
// GET /api/v1/drivers/:id/reviews
func (c *RatingController) GetDriverReviews(ctx *gin.Context) {
	driverID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid driver ID", err.Error()))
		return
	}

	page := parseInt(ctx.DefaultQuery("page", "1"), 1)
	limit := parseInt(ctx.DefaultQuery("per_page", "10"), 10)

	ratings, total, err := c.service.GetDriverReviews(driverID, page, limit)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to get reviews", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Reviews retrieved", gin.H{
		"reviews": ratings,
		"meta": gin.H{
			"page":        page,
			"per_page":    limit,
			"total":       total,
			"total_pages": (total + int64(limit) - 1) / int64(limit),
		},
	}))
}

// GetDriverRatingSummary gets rating summary for a driver
// GET /api/v1/drivers/:id/rating-summary
func (c *RatingController) GetDriverRatingSummary(ctx *gin.Context) {
	driverID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid driver ID", err.Error()))
		return
	}

	summary, err := c.service.GetDriverRatingSummary(driverID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to get rating summary", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Rating summary retrieved", summary))
}

// ReportRatingRequest represents rating report request
type ReportRatingRequest struct {
	Reason  string `json:"reason" binding:"required"`
	Details string `json:"details"`
}

// ReportRating reports a rating for review
// POST /api/v1/ratings/:id/report
func (c *RatingController) ReportRating(ctx *gin.Context) {
	ratingID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid rating ID", err.Error()))
		return
	}

	var req ReportRatingRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	if err := c.service.ReportRating(ratingID, req.Reason, req.Details); err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to report rating", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Rating reported successfully", nil))
}

// GetMyRideRating gets the current user's rating for a specific ride
// GET /api/v1/rides/:id/my-rating
func (c *RatingController) GetMyRideRating(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	uid, _ := uuid.Parse(userID)
	userType := ctx.GetString("userType")

	rideID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid ride ID", err.Error()))
		return
	}

	// Get rating for this ride
	rating, err := c.service.GetRideRating(rideID, uid, userType)
	if err != nil {
		ctx.JSON(http.StatusNotFound, utils.ErrorResponse("Rating not found", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Rating retrieved", rating))
}

// GetDriverRatingStats gets comprehensive rating stats for a driver
// GET /api/v1/drivers/:id/rating-stats
func (c *RatingController) GetDriverRatingStats(ctx *gin.Context) {
	driverID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid driver ID", err.Error()))
		return
	}

	stats := c.service.GetRatingStatsForDriver(driverID)

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Rating stats retrieved", stats))
}

// AdminGetPendingReports gets all reported ratings pending review
// GET /api/v1/admin/ratings/reports
func (c *RatingController) AdminGetPendingReports(ctx *gin.Context) {
	page := parseInt(ctx.DefaultQuery("page", "1"), 1)
	limit := parseInt(ctx.DefaultQuery("per_page", "10"), 10)

	reports, total, err := c.service.GetPendingReports(page, limit)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to get reports", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Reports retrieved", gin.H{
		"reports": reports,
		"meta": gin.H{
			"page":        page,
			"per_page":    limit,
			"total":       total,
			"total_pages": (total + int64(limit) - 1) / int64(limit),
		},
	}))
}

// AdminResolveReport resolves a reported rating
// POST /api/v1/admin/ratings/reports/:id/resolve
func (c *RatingController) AdminResolveReport(ctx *gin.Context) {
	ratingID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid rating ID", err.Error()))
		return
	}

	var req struct {
		Action string `json:"action" binding:"required,oneof=remove keep"`
		Notes  string `json:"notes"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	if err := c.service.ResolveReport(ratingID, req.Action, req.Notes); err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to resolve report", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Report resolved successfully", nil))
}

// parseInt helper function for parsing int from string
func parseInt(s string, defaultVal int) int {
	if s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			return v
		}
	}
	return defaultVal
}
