package controllers

import (
	"io"
	"net/http"

	"rapido-backend/services"
	"rapido-backend/utils"

	"github.com/gin-gonic/gin"
)

// CRMWebhookController receives CRM webhook events from external systems.
type CRMWebhookController struct {
	Service *services.CRMWebhookService
}

// NewCRMWebhookController creates a CRM webhook controller.
func NewCRMWebhookController() *CRMWebhookController {
	return &CRMWebhookController{Service: services.NewCRMWebhookService()}
}

// HandleWebhook validates and acknowledges CRM webhook payloads.
func (c *CRMWebhookController) HandleWebhook(ctx *gin.Context) {
	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Failed to read webhook body", err.Error()))
		return
	}

	event, duplicate, err := c.Service.ProcessWebhook(body, ctx.Request.Header)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Invalid webhook", err.Error()))
		return
	}

	status := http.StatusAccepted
	message := "Webhook accepted"
	if duplicate {
		status = http.StatusOK
		message = "Webhook already processed"
	}

	ctx.JSON(status, utils.SuccessResponse(message, map[string]interface{}{
		"event_id":    event.EventID,
		"event":       event.Event,
		"entity_type": event.EntityType,
		"entity_id":   event.EntityID,
		"duplicate":   duplicate,
	}))
}
