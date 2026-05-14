package crm

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
)

const (
	EventVersion      = "1.0"
	DefaultSource     = "rapido-backend"
	DefaultWebhookURL = "/webhooks/rapido"

	HeaderEventID    = "X-Rapido-Event-ID"
	HeaderEvent      = "X-Rapido-Event"
	HeaderVersion    = "X-Rapido-Event-Version"
	HeaderSource     = "X-Rapido-Source"
	HeaderTimestamp  = "X-Rapido-Timestamp"
	HeaderRetryCount = "X-Rapido-Retry-Count"
	HeaderSignature  = "X-Rapido-Signature"
)

// EventEnvelope is the canonical webhook payload exchanged with the external CRM.
type EventEnvelope struct {
	Version    string                 `json:"version"`
	EventID    string                 `json:"event_id"`
	Event      string                 `json:"event"`
	Source     string                 `json:"source"`
	EntityType string                 `json:"entity_type"`
	EntityID   string                 `json:"entity_id"`
	OccurredAt time.Time              `json:"occurred_at"`
	RetryCount int                    `json:"retry_count"`
	Data       map[string]interface{} `json:"data,omitempty"`
}

// BuildEventEnvelope constructs a standard CRM event envelope.
func BuildEventEnvelope(event, entityType, entityID string, data map[string]interface{}, retryCount int) EventEnvelope {
	if data == nil {
		data = map[string]interface{}{}
	}

	now := time.Now().UTC()

	return EventEnvelope{
		Version:    EventVersion,
		EventID:    uuid.NewString(),
		Event:      event,
		Source:     DefaultSource,
		EntityType: entityType,
		EntityID:   entityID,
		OccurredAt: now,
		RetryCount: retryCount,
		Data:       data,
	}
}

// SignWebhook returns an HMAC signature of timestamp + "." + body.
func SignWebhook(body []byte, secret, timestamp string) string {
	if secret == "" {
		return ""
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyWebhookSignature verifies the HMAC signature for the given payload.
func VerifyWebhookSignature(body []byte, secret, timestamp, signature string) bool {
	if secret == "" {
		return false
	}

	computed := SignWebhook(body, secret, timestamp)
	return hmac.Equal([]byte(computed), []byte(signature))
}
