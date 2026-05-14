package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"rapido-backend/config"
	"rapido-backend/database"
	crm "rapido-backend/integrations/crm"
)

// CRMWebhookService validates and deduplicates CRM webhook events.
type CRMWebhookService struct{}

// NewCRMWebhookService creates a CRM webhook service.
func NewCRMWebhookService() *CRMWebhookService {
	return &CRMWebhookService{}
}

// ProcessWebhook validates, deduplicates and acknowledges an incoming CRM event.
func (s *CRMWebhookService) ProcessWebhook(body []byte, headers http.Header) (*crm.EventEnvelope, bool, error) {
	timestamp := headers.Get(crm.HeaderTimestamp)
	signature := headers.Get(crm.HeaderSignature)
	cfg := config.Get().CRM

	if cfg.WebhookSecret != "" {
		if timestamp == "" || signature == "" {
			return nil, false, errors.New("missing webhook signature headers")
		}
		if !crm.VerifyWebhookSignature(body, cfg.WebhookSecret, timestamp, signature) {
			return nil, false, errors.New("invalid webhook signature")
		}
	} else {
		log.Println("Warning: CRM webhook secret not configured; accepting request in development mode")
	}

	var event crm.EventEnvelope
	if err := json.Unmarshal(body, &event); err != nil {
		return nil, false, err
	}

	if event.EventID == "" || event.Event == "" || event.Version == "" {
		return nil, false, errors.New("invalid event envelope")
	}

	if headerEvent := headers.Get(crm.HeaderEvent); headerEvent != "" && headerEvent != event.Event {
		return nil, false, fmt.Errorf("event header mismatch: %s", headerEvent)
	}
	if headerEventID := headers.Get(crm.HeaderEventID); headerEventID != "" && headerEventID != event.EventID {
		return nil, false, fmt.Errorf("event id header mismatch: %s", headerEventID)
	}

	receivedKey := "crm:webhook:processed:" + event.EventID
	if database.RedisClient != nil {
		if cached, err := database.GetCache(receivedKey); err == nil && cached == "processed" {
			return &event, true, nil
		}
	}

	if database.RedisClient != nil {
		_ = database.SetCache(receivedKey, "processed", 24*time.Hour)
	}

	return &event, false, nil
}
