package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
	Server   ServerConfig
	CRM      CRMConfig
	Twilio   TwilioConfig
	FCM      FCMConfig
	Google   GoogleConfig
	Razorpay RazorpayConfig
	App      AppConfig
	Admin    AdminConfig
}

type AdminConfig struct {
	Email    string
	Password string
	Name     string
	Phone    string
}

type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string
}

type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

type JWTConfig struct {
	Secret        string
	RefreshSecret string
	AccessExpiry  time.Duration
	RefreshExpiry time.Duration
}

type ServerConfig struct {
	Port string
	Mode string
}

type CRMConfig struct {
	Enabled       bool
	BaseURL       string
	Token         string
	WebhookSecret string
	WebhookAPIKey string
	AllowedIPs    string
	TimeoutS      int
}

type TwilioConfig struct {
	AccountSID  string
	AuthToken   string
	PhoneNumber string
}

type FCMConfig struct {
	ServerKey       string
	CredentialsFile string
}

type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	APIKey       string // For Google Maps API
}

type RazorpayConfig struct {
	KeyID         string
	KeySecret     string
	WebhookSecret string
}

type AppConfig struct {
	DefaultCurrency           string
	PlatformCommissionPercent float64
	DriverSearchRadiusKM      float64
	RideRequestTimeoutSec     int
	OTPExpiryMin              int
	Environment               string  // development, staging, production
	DevTestOTP                string  // bypass OTP for dev only — set DEV_TEST_OTP in .env
	InvoiceGSTPercent         float64 // GST percentage for invoices (default 18)
	CancellationFreeWindowSec int     // seconds after which cancellation fee applies
	CancellationFlatFee       float64 // flat fee before driver assigned
	CancellationAfterAssigned float64 // % of base fare after driver assigned
	CancellationAfterArrived  float64 // % of base fare after driver arrived
}

var AppConfigInstance *Config

func Load() *Config {
	// Try to find .env file in multiple locations
	envPaths := []string{
		".env",       // Current directory
		"../.env",    // Parent directory
		"../../.env", // Grandparent directory
		filepath.Join(getExecutableDir(), ".env"), // Same dir as executable
	}

	viper.AutomaticEnv()

	loaded := false
	for _, path := range envPaths {
		if _, err := os.Stat(path); err == nil {
			viper.SetConfigFile(path)
			if err := viper.ReadInConfig(); err == nil {
				log.Printf("Loaded config from: %s", path)
				loaded = true
				break
			}
		}
	}

	if !loaded {
		log.Println("No .env file found, using environment variables only")
	}

	accessExpiry := time.Duration(viper.GetInt("JWT_ACCESS_EXPIRY_MINUTES")) * time.Minute
	if accessExpiry == 0 {
		accessExpiry = 15 * time.Minute
	}

	refreshExpiry := time.Duration(viper.GetInt("JWT_REFRESH_EXPIRY_DAYS")) * 24 * time.Hour
	if refreshExpiry == 0 {
		refreshExpiry = 7 * 24 * time.Hour
	}

	AppConfigInstance = &Config{
		Database: DatabaseConfig{
			Host:     viper.GetString("DB_HOST"),
			Port:     viper.GetInt("DB_PORT"),
			User:     viper.GetString("DB_USERNAME"),
			Password: viper.GetString("DB_PASSWORD"),
			Name:     viper.GetString("DB_DATABASE"),
			SSLMode:  viper.GetString("DB_SSLMODE"),
		},
		Redis: RedisConfig{
			Host:     viper.GetString("REDIS_ADDR"),
			Port:     viper.GetInt("REDIS_PORT"),
			Password: viper.GetString("REDIS_PASSWORD"),
			DB:       viper.GetInt("REDIS_DB"),
		},
		JWT: JWTConfig{
			Secret:        viper.GetString("JWT_SECRET"),
			RefreshSecret: viper.GetString("JWT_REFRESH_SECRET"),
			AccessExpiry:  accessExpiry,
			RefreshExpiry: refreshExpiry,
		},
		Server: ServerConfig{
			Port: viper.GetString("SERVER_PORT"),
			Mode: viper.GetString("GIN_MODE"),
		},
		CRM: CRMConfig{
			Enabled:       viper.GetBool("EXTERNAL_CRM_ENABLED"),
			BaseURL:       viper.GetString("EXTERNAL_CRM_BASE_URL"),
			Token:         viper.GetString("EXTERNAL_CRM_TOKEN"),
			WebhookSecret: viper.GetString("EXTERNAL_CRM_WEBHOOK_SECRET"),
			WebhookAPIKey: viper.GetString("EXTERNAL_CRM_WEBHOOK_API_KEY"),
			AllowedIPs:    viper.GetString("EXTERNAL_CRM_WEBHOOK_ALLOWED_IPS"),
			TimeoutS:      viper.GetInt("EXTERNAL_CRM_TIMEOUT_SECONDS"),
		},
		Twilio: TwilioConfig{
			AccountSID:  viper.GetString("TWILIO_ACCOUNT_SID"),
			AuthToken:   viper.GetString("TWILIO_AUTH_TOKEN"),
			PhoneNumber: viper.GetString("TWILIO_FROM_NUMBER"),
		},
		FCM: FCMConfig{
			ServerKey: viper.GetString("FCM_SERVER_KEY"),
		},
		Google: GoogleConfig{
			ClientID:     viper.GetString("GOOGLE_CLIENT_ID"),
			ClientSecret: viper.GetString("GOOGLE_CLIENT_SECRET"),
		},
		Razorpay: RazorpayConfig{
			KeyID:         viper.GetString("RAZORPAY_KEY_ID"),
			KeySecret:     viper.GetString("RAZORPAY_KEY_SECRET"),
			WebhookSecret: viper.GetString("RAZORPAY_WEBHOOK_SECRET"),
		},
		App: AppConfig{
			DefaultCurrency:           viper.GetString("DEFAULT_CURRENCY"),
			PlatformCommissionPercent: viper.GetFloat64("PLATFORM_COMMISSION_PERCENT"),
			DriverSearchRadiusKM:      viper.GetFloat64("DRIVER_SEARCH_RADIUS_KM"),
			RideRequestTimeoutSec:     viper.GetInt("RIDE_REQUEST_TIMEOUT_SECONDS"),
			OTPExpiryMin:              viper.GetInt("OTP_EXPIRY_MINUTES"),
			Environment:               viper.GetString("APP_ENV"),
			DevTestOTP:                viper.GetString("DEV_TEST_OTP"),
			InvoiceGSTPercent:         viper.GetFloat64("INVOICE_GST_PERCENT"),
			CancellationFreeWindowSec: viper.GetInt("CANCELLATION_FREE_WINDOW_SEC"),
			CancellationFlatFee:       viper.GetFloat64("CANCELLATION_FLAT_FEE"),
			CancellationAfterAssigned: viper.GetFloat64("CANCELLATION_AFTER_ASSIGNED_PERCENT"),
			CancellationAfterArrived:  viper.GetFloat64("CANCELLATION_AFTER_ARRIVED_PERCENT"),
		},
		Admin: AdminConfig{
			Email:    viper.GetString("ADMIN_EMAIL"),
			Password: viper.GetString("ADMIN_PASSWORD"),
			Name:     viper.GetString("ADMIN_NAME"),
			Phone:    viper.GetString("ADMIN_PHONE"),
		},
	}

	return AppConfigInstance
}

// getExecutableDir returns the directory where the executable is located
func getExecutableDir() string {
	ex, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(ex)
}

func Get() *Config {
	if AppConfigInstance == nil {
		return Load()
	}
	return AppConfigInstance
}

// Validate checks for required production settings and refuses weak defaults.
func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}

	env := strings.ToLower(strings.TrimSpace(c.App.Environment))
	if env != "production" && env != "staging" {
		return nil
	}

	required := map[string]string{
		"DB_HOST":                 c.Database.Host,
		"DB_USERNAME":             c.Database.User,
		"DB_PASSWORD":             c.Database.Password,
		"DB_DATABASE":             c.Database.Name,
		"JWT_SECRET":              c.JWT.Secret,
		"JWT_REFRESH_SECRET":      c.JWT.RefreshSecret,
		"RAZORPAY_KEY_ID":         c.Razorpay.KeyID,
		"RAZORPAY_KEY_SECRET":     c.Razorpay.KeySecret,
		"RAZORPAY_WEBHOOK_SECRET": c.Razorpay.WebhookSecret,
		"ADMIN_EMAIL":             c.Admin.Email,
		"ADMIN_PASSWORD":          c.Admin.Password,
		"ADMIN_NAME":              c.Admin.Name,
		"ADMIN_PHONE":             c.Admin.Phone,
	}

	for name, value := range required {
		if isMissingOrPlaceholder(value) {
			return fmt.Errorf("%s must be set for %s", name, env)
		}
	}

	if isMissingOrPlaceholder(c.Redis.Host) && c.Redis.Port == 0 {
		return fmt.Errorf("REDIS_ADDR or REDIS_PORT must be set for %s", env)
	}

	twilioConfigured := !isMissingOrPlaceholder(c.Twilio.AccountSID) && !isMissingOrPlaceholder(c.Twilio.AuthToken) && !isMissingOrPlaceholder(c.Twilio.PhoneNumber)
	msg91Configured := !isMissingOrPlaceholder(os.Getenv("MSG91_AUTH_KEY"))
	if !twilioConfigured && !msg91Configured {
		return fmt.Errorf("an SMS provider must be configured for %s", env)
	}

	if !isMissingOrPlaceholder(c.App.DevTestOTP) {
		return fmt.Errorf("DEV_TEST_OTP must be empty outside development")
	}

	return nil
}

func isMissingOrPlaceholder(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return true
	}

	lower := strings.ToLower(trimmed)
	return strings.Contains(lower, "change-me") || strings.Contains(lower, "replace-me") || strings.Contains(lower, "placeholder")
}
