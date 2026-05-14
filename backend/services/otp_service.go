package services

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"

	"rapido-backend/config"
	"rapido-backend/database"
	"rapido-backend/utils"
)

const (
	OTPKeyPrefix     = "otp:"
	OTPAttemptPrefix = "otp_attempts:"
	MaxOTPAttempts   = 5
)

// OTPService handles OTP generation and verification using Redis
type OTPService struct{}

// NewOTPService creates a new OTP service
func NewOTPService() *OTPService {
	return &OTPService{}
}

// isDevMode returns true when APP_ENV is explicitly "development" or "dev".
func isDevMode() bool {
	env := strings.ToLower(strings.TrimSpace(config.Get().App.Environment))
	return env == "development" || env == "dev"
}

// devTestOTP returns the configurable dev bypass OTP from DEV_TEST_OTP env var.
// Falls back to empty string (disabled) when not set.
func devTestOTP() string {
	return strings.TrimSpace(config.Get().App.DevTestOTP)
}

// GenerateAndStoreOTP generates a secure OTP, stores it in Redis (hashed),
// and prints it to the terminal when running in development mode.
func (s *OTPService) GenerateAndStoreOTP(phone, purpose string) (string, error) {
	// Generate OTP
	code := utils.GenerateOTP(6)

	// Hash the OTP (never store plain text)
	hashedCode := hashOTP(code, phone)

	// Get expiry from config
	otpExpiry := time.Duration(config.Get().App.OTPExpiryMin) * time.Minute

	// Store in Redis with TTL
	key := OTPKeyPrefix + phone + ":" + purpose
	if err := database.SetCache(key, hashedCode, otpExpiry); err != nil {
		return "", fmt.Errorf("failed to store OTP: %w", err)
	}

	// Reset attempt counter
	attemptKey := OTPAttemptPrefix + phone
	database.SetCache(attemptKey, "0", otpExpiry)

	// Print OTP to terminal in development mode (never in production)
	if isDevMode() {
		log.Printf("[DEV] OTP for %s (%s): %s  (expires in %v)", phone, purpose, code, otpExpiry)
	}

	return code, nil
}

// ClearOTP removes any stored OTP state for the given phone and purpose.
func (s *OTPService) ClearOTP(phone, purpose string) {
	key := OTPKeyPrefix + phone + ":" + purpose
	attemptKey := OTPAttemptPrefix + phone
	_ = database.DeleteCache(key)
	_ = database.DeleteCache(attemptKey)
}

// VerifyOTP validates OTP from Redis and implements rate limiting.
// In development mode a configurable bypass OTP (DEV_TEST_OTP env var) is also accepted.
func (s *OTPService) VerifyOTP(phone, code, purpose string) error {
	// Development bypass: accept DEV_TEST_OTP only when APP_ENV=development
	if isDevMode() {
		bypass := devTestOTP()
		if bypass != "" && code == bypass {
			// Clear any existing OTP for this phone to prevent conflicts
			key := OTPKeyPrefix + phone + ":" + purpose
			attemptKey := OTPAttemptPrefix + phone
			database.DeleteCache(key)
			database.DeleteCache(attemptKey)
			log.Printf("[DEV] OTP bypass used for %s (%s)", phone, purpose)
			return nil
		}
	}

	// Check attempt count (rate limiting)
	attemptKey := OTPAttemptPrefix + phone
	attemptsStr, _ := database.GetCache(attemptKey)
	attempts := 0
	fmt.Sscanf(attemptsStr, "%d", &attempts)

	if attempts >= MaxOTPAttempts {
		return fmt.Errorf("too many failed attempts, please request new OTP")
	}

	// Get stored hash from Redis
	key := OTPKeyPrefix + phone + ":" + purpose
	storedHash, err := database.GetCache(key)
	if err != nil {
		return fmt.Errorf("OTP not found - may be expired or Redis unavailable: %v", err)
	}
	if storedHash == "" {
		return fmt.Errorf("OTP expired or already used")
	}

	// Hash the provided code and compare
	providedHash := hashOTP(code, phone)
	if providedHash != storedHash {
		// Increment attempt counter
		attempts++
		database.SetCache(attemptKey, fmt.Sprintf("%d", attempts), 10*time.Minute)
		return fmt.Errorf("invalid OTP")
	}

	// Success - delete OTP from Redis (one-time use)
	database.DeleteCache(key)
	database.DeleteCache(attemptKey)

	return nil
}

// hashOTP creates a SHA-256 hash of the OTP with phone as salt
func hashOTP(code, phone string) string {
	hasher := sha256.New()
	hasher.Write([]byte(code + ":" + phone + ":" + config.Get().JWT.Secret))
	return hex.EncodeToString(hasher.Sum(nil))
}

// IsOTPValid checks if an OTP exists and is valid (without consuming it)
func (s *OTPService) IsOTPValid(phone, purpose string) bool {
	key := OTPKeyPrefix + phone + ":" + purpose
	value, err := database.GetCache(key)
	return err == nil && value != ""
}
