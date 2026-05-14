package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"rapido-backend/config"
)

func main() {
	// Allow override of APP_ENV for testing
	appEnv := flag.String("env", os.Getenv("APP_ENV"), "APP_ENV to validate (default: from environment)")
	flag.Parse()

	if *appEnv != "" {
		os.Setenv("APP_ENV", *appEnv)
	}

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "production"
	}

	fmt.Printf("=== Rapido Backend Production Config Validator ===\n")
	fmt.Printf("Environment: %s\n\n", env)

	// Load config (this will use .env if available, then environment variables)
	cfg := config.Load()

	// Validate production settings
	if err := cfg.Validate(); err != nil {
		log.Printf("❌ Configuration validation failed for %s: %v\n", env, err)
		os.Exit(1)
	}

	log.Printf("✓ Configuration valid for %s\n", env)

	// Display loaded config (sanitized)
	fmt.Printf("\nLoaded Configuration Summary:\n")
	fmt.Printf("  Database: %s @ %s:%d\n", cfg.Database.Name, cfg.Database.Host, cfg.Database.Port)
	fmt.Printf("  Redis: %s\n", cfg.Redis.Host)
	fmt.Printf("  JWT Expiry: %v\n", cfg.JWT.AccessExpiry)
	fmt.Printf("  Admin Email: %s\n", cfg.Admin.Email)
	fmt.Printf("  Server Port: %s\n", cfg.Server.Port)
	fmt.Printf("  Gin Mode: %s\n", cfg.Server.Mode)

	// Check providers
	hasTwilio := cfg.Twilio.AccountSID != "" && cfg.Twilio.AuthToken != ""
	hasMSG91 := os.Getenv("MSG91_AUTH_KEY") != ""
	hasRazorpay := cfg.Razorpay.KeyID != "" && cfg.Razorpay.KeySecret != ""

	fmt.Printf("\nProviders:\n")
	if hasTwilio {
		fmt.Printf("  ✓ Twilio SMS\n")
	}
	if hasMSG91 {
		fmt.Printf("  ✓ MSG91 SMS\n")
	}
	if hasRazorpay {
		fmt.Printf("  ✓ Razorpay Payments\n")
	}
	if !hasTwilio && !hasMSG91 {
		fmt.Printf("  ✗ No SMS provider (invalid in production)\n")
	}
	if !hasRazorpay {
		fmt.Printf("  ✓ Razorpay (optional: can be disabled)\n")
	}

	fmt.Printf("\n✅ Backend is ready for deployment\n")
	os.Exit(0)
}
