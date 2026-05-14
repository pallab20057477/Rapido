package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"rapido-backend/config"
	"rapido-backend/database"
	"rapido-backend/utils"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	// WebhookIDPrefix for deduplication
	WebhookIDPrefix = "webhook:id:"
	// WebhookReplayWindow max age of webhook (5 minutes)
	WebhookReplayWindow = 5 * time.Minute
)

// WebhookSecurityMiddleware adds security checks for webhooks:
// - API key validation
// - IP allowlist
// - Replay protection (deduplication)
// - Timestamp validation
// - HMAC signature verification
func WebhookSecurityMiddleware() gin.HandlerFunc {
	cfg := config.Get().CRM
	apiKey := strings.TrimSpace(cfg.WebhookAPIKey)
	allowList := parseAllowList(cfg.AllowedIPs)
	webhookSecret := strings.TrimSpace(cfg.WebhookSecret)

	return func(c *gin.Context) {
		// Read body for signature verification
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			utils.Error("failed to read webhook body", utils.ErrorLogFields(err, "webhook_error", c.Request.URL.Path)...)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
			c.Abort()
			return
		}

		// Store body for later controllers
		c.Set("webhook_body", body)

		// API Key validation
		if apiKey != "" {
			incomingKey := strings.TrimSpace(c.GetHeader("X-API-Key"))
			if incomingKey == "" {
				incomingKey = strings.TrimSpace(c.GetHeader("X-Rapido-Webhook-Key"))
			}
			if incomingKey != apiKey {
				utils.Warn("webhook rejected - invalid API key", zap.String("ip", c.ClientIP()))
				c.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", "invalid webhook api key"))
				c.Abort()
				return
			}
		}

		// IP allowlist check
		if len(allowList) > 0 {
			ip := net.ParseIP(c.ClientIP())
			if ip == nil || !isAllowedIP(ip, allowList) {
				utils.Warn("webhook rejected - IP not allowed", zap.String("ip", c.ClientIP()))
				c.JSON(http.StatusForbidden, utils.ErrorResponse("Forbidden", "source ip not allowed"))
				c.Abort()
				return
			}
		}

		// Replay protection - check webhook ID
		webhookID := c.GetHeader("X-Webhook-ID")
		if webhookID == "" {
			webhookID = c.GetHeader("X-Request-ID")
		}
		if webhookID == "" {
			webhookID = c.GetHeader("X-Event-ID")
		}

		if webhookID != "" {
			if isDuplicateWebhook(webhookID) {
				utils.Info("duplicate webhook received - already processed",
					zap.String("webhook_id", webhookID),
					zap.String("path", c.Request.URL.Path),
				)
				c.JSON(http.StatusOK, gin.H{"status": "already processed"})
				c.Abort()
				return
			}
			storeWebhookID(webhookID, 24*time.Hour)
		}

		// Timestamp validation (prevent replay of old webhooks)
		timestamp := c.GetHeader("X-Webhook-Timestamp")
		if timestamp != "" {
			if isWebhookExpired(timestamp) {
				utils.Warn("webhook rejected - too old",
					zap.String("timestamp", timestamp),
					zap.String("path", c.Request.URL.Path),
				)
				c.JSON(http.StatusUnauthorized, gin.H{"error": "webhook expired"})
				c.Abort()
				return
			}
		}

		// HMAC signature verification
		if webhookSecret != "" {
			signature := c.GetHeader("X-Webhook-Signature")
			if signature == "" {
				signature = c.GetHeader("X-Signature")
			}
			if signature != "" && !verifyWebhookSignature(body, signature, webhookSecret) {
				utils.Warn("webhook signature verification failed",
					zap.String("path", c.Request.URL.Path),
					zap.String("ip", c.ClientIP()),
				)
				c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
				c.Abort()
				return
			}
		}

		utils.Info("webhook received",
			zap.String("webhook_id", webhookID),
			zap.String("path", c.Request.URL.Path),
			zap.String("ip", c.ClientIP()),
		)

		c.Next()

		// Log result
		status := c.Writer.Status()
		if status >= 200 && status < 300 {
			utils.Info("webhook processed successfully",
				zap.String("webhook_id", webhookID),
				zap.Int("status", status),
			)
		} else {
			utils.Warn("webhook processing failed",
				zap.String("webhook_id", webhookID),
				zap.Int("status", status),
			)
		}
	}
}

type allowRule struct {
	ipNet *net.IPNet
	ip    net.IP
}

func parseAllowList(raw string) []allowRule {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	rules := make([]allowRule, 0, len(parts))
	for _, part := range parts {
		entry := strings.TrimSpace(part)
		if entry == "" {
			continue
		}

		if strings.Contains(entry, "/") {
			_, ipNet, err := net.ParseCIDR(entry)
			if err == nil {
				rules = append(rules, allowRule{ipNet: ipNet})
			}
			continue
		}

		if ip := net.ParseIP(entry); ip != nil {
			rules = append(rules, allowRule{ip: ip})
		}
	}
	return rules
}

func isAllowedIP(ip net.IP, rules []allowRule) bool {
	for _, rule := range rules {
		if rule.ipNet != nil && rule.ipNet.Contains(ip) {
			return true
		}
		if rule.ip != nil && rule.ip.Equal(ip) {
			return true
		}
	}
	return false
}

// Webhook replay protection functions
func isDuplicateWebhook(webhookID string) bool {
	if database.RedisClient == nil {
		return false
	}
	ctx := context.Background()
	key := WebhookIDPrefix + webhookID
	exists, err := database.RedisClient.Exists(ctx, key).Result()
	if err != nil {
		return false
	}
	return exists > 0
}

func storeWebhookID(webhookID string, ttl time.Duration) {
	if database.RedisClient == nil {
		return
	}
	ctx := context.Background()
	key := WebhookIDPrefix + webhookID
	database.RedisClient.Set(ctx, key, time.Now().Unix(), ttl)
}

func isWebhookExpired(timestamp string) bool {
	ts, err := parseWebhookTimestamp(timestamp)
	if err != nil {
		return true
	}
	age := time.Since(ts)
	return age > WebhookReplayWindow || age < -1*time.Minute
}

func parseWebhookTimestamp(ts string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		time.UnixDate,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05Z",
	}
	var unixTs int64
	if _, err := fmt.Sscanf(ts, "%d", &unixTs); err == nil {
		if unixTs > 1000000000000 {
			return time.Unix(0, unixTs*int64(time.Millisecond)), nil
		}
		return time.Unix(unixTs, 0), nil
	}
	for _, format := range formats {
		if t, err := time.Parse(format, ts); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse timestamp: %s", ts)
}

func verifyWebhookSignature(body []byte, signature, secret string) bool {
	if strings.Contains(signature, "=") {
		parts := strings.SplitN(signature, "=", 2)
		if len(parts) == 2 {
			signature = parts[1]
		}
	}
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(body)
	expectedSig := hex.EncodeToString(h.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expectedSig))
}

// WebhookBody returns the webhook body from context
func WebhookBody(c *gin.Context) []byte {
	if body, exists := c.Get("webhook_body"); exists {
		return body.([]byte)
	}
	return nil
}
