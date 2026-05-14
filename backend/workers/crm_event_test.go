package workers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	crm "rapido-backend/integrations/crm"
)

func TestBuildCRMEventEnvelope(t *testing.T) {
	env := crm.BuildEventEnvelope("user.upserted", "user", "123", map[string]interface{}{
		"name": "Asha",
	}, 2)

	if env.Version != crm.EventVersion {
		t.Fatalf("expected version %q, got %q", crm.EventVersion, env.Version)
	}
	if env.Event == "" || env.EventID == "" || env.Source == "" {
		t.Fatalf("expected event metadata to be populated: %+v", env)
	}
	if env.EntityType != "user" || env.EntityID != "123" {
		t.Fatalf("unexpected entity metadata: %+v", env)
	}
	if env.RetryCount != 2 {
		t.Fatalf("expected retry count 2, got %d", env.RetryCount)
	}
}

func TestSignCRMWebhook(t *testing.T) {
	body := []byte(`{"hello":"world"}`)
	secret := "top-secret"
	timestamp := "2026-05-01T00:00:00Z"

	sig := crm.SignWebhook(body, secret, timestamp)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	want := hex.EncodeToString(mac.Sum(nil))

	if sig != want {
		t.Fatalf("unexpected signature: got %s want %s", sig, want)
	}
}
