package controllers

import (
	"net/http"
	"rapido-backend/services"
	"rapido-backend/utils"

	"github.com/gin-gonic/gin"
)

type ConfigController struct {
	Service *services.ConfigService
}

func NewConfigController() *ConfigController {
	return &ConfigController{
		Service: services.NewConfigService(),
	}
}

// GetPublicConfig returns public app configuration for clients
func (c *ConfigController) GetPublicConfig(ctx *gin.Context) {
	config := c.Service.GetPublicConfig()
	ctx.JSON(http.StatusOK, utils.SuccessResponse("Config retrieved", config))
}

// GetCancellationReasons returns list of valid cancellation reasons
func (c *ConfigController) GetCancellationReasons(ctx *gin.Context) {
	reasons := []map[string]interface{}{
		{"code": "rider_cancelled", "label": "Rider cancelled", "applies_to": "rider"},
		{"code": "driver_cancelled", "label": "Driver cancelled", "applies_to": "driver"},
		{"code": "no_driver_found", "label": "No driver found", "applies_to": "system"},
		{"code": "wrong_pickup_location", "label": "Wrong pickup location", "applies_to": "rider"},
		{"code": "pickup_too_far", "label": "Pickup location too far", "applies_to": "driver"},
		{"code": "rider_no_show", "label": "Rider didn't show up", "applies_to": "driver"},
		{"code": "driver_no_show", "label": "Driver didn't show up", "applies_to": "rider"},
		{"code": "vehicle_issue", "label": "Vehicle issue", "applies_to": "driver"},
		{"code": "other", "label": "Other", "applies_to": "all"},
	}
	ctx.JSON(http.StatusOK, utils.SuccessResponse("Cancellation reasons", map[string]interface{}{
		"reasons": reasons,
	}))
}

// GetConfig returns unified config - public for all, full for admin
func (c *ConfigController) GetConfig(ctx *gin.Context) {
	// Check if user is authenticated
	userID, exists := ctx.Get("user_id")
	var isAdmin bool

	if exists {
		// Check if admin
		if role, roleExists := ctx.Get("role"); roleExists && role == "admin" {
			isAdmin = true
		}

		// Update last known location if provided (optional tracking)
		if lat := ctx.Query("lat"); lat != "" {
			if lng := ctx.Query("lng"); lng != "" {
				// Could store user's last known location for analytics
				_ = userID
				_ = lat
				_ = lng
			}
		}
	}

	if isAdmin {
		// Return full config for admin
		publicConfig := c.Service.GetPublicConfig()
		systemConfig := c.Service.GetSystemConfig()
		ctx.JSON(http.StatusOK, utils.SuccessResponse("Full config retrieved", map[string]interface{}{
			"public": publicConfig,
			"system": systemConfig,
		}))
		return
	}

	// Return public config for regular users and unauthenticated requests
	config := c.Service.GetPublicConfig()
	ctx.JSON(http.StatusOK, utils.SuccessResponse("Config retrieved", config))
}
