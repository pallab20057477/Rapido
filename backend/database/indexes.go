package database

import (
	"fmt"
	"rapido-backend/utils"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// CreateProductionIndexes creates all required indexes for production
func CreateProductionIndexes(db *gorm.DB) error {
	utils.Info("Creating production database indexes...")
	
	indexes := []struct {
		Name    string
		Table   string
		Columns string
		Unique  bool
	}{
		// Rides table indexes
		{"idx_rides_status", "rides", "status", false},
		{"idx_rides_driver_id", "rides", "driver_id", false},
		{"idx_rides_rider_id", "rides", "rider_id", false},
		{"idx_rides_created_at", "rides", "created_at", false},
		{"idx_rides_status_created", "rides", "status, created_at", false},
		{"idx_rides_vehicle_type", "rides", "vehicle_type", false},
		{"idx_rides_payment_status", "rides", "payment_status", false},
		
		// Drivers table indexes
		{"idx_drivers_is_online", "drivers", "is_online", false},
		{"idx_drivers_is_verified", "drivers", "is_verified", false},
		{"idx_drivers_rating", "drivers", "rating", false},
		{"idx_drivers_vehicle_type", "drivers", "vehicle_type", false},
		{"idx_drivers_online_verified", "drivers", "is_online, is_verified", false},
		
		// Users table indexes
		{"idx_users_phone", "users", "phone", true},
		{"idx_users_email", "users", "email", true},
		{"idx_users_role", "users", "role", false},
		
		// Driver locations indexes (for geo queries)
		{"idx_driver_locations_driver_id", "driver_locations", "driver_id", true},
		{"idx_driver_locations_updated", "driver_locations", "last_updated", false},
		
		// Payments table indexes
		{"idx_payments_ride_id", "payments", "ride_id", false},
		{"idx_payments_user_id", "payments", "user_id", false},
		{"idx_payments_status", "payments", "status", false},
		{"idx_payments_gateway_ref", "payments", "gateway_ref", false},
		{"idx_payments_created_at", "payments", "created_at", false},
		
		// Transactions table indexes
		{"idx_transactions_user_id", "transactions", "user_id", false},
		{"idx_transactions_type", "transactions", "type", false},
		{"idx_transactions_created_at", "transactions", "created_at", false},
		
		// Notifications indexes
		{"idx_notifications_user_id", "notifications", "user_id", false},
		{"idx_notifications_status", "notifications", "status", false},
		{"idx_notifications_user_status", "notifications", "user_id, status", false},
		
		// Ride locations indexes
		{"idx_ride_locations_ride_id", "ride_locations", "ride_id", false},
		{"idx_ride_locations_timestamp", "ride_locations", "timestamp", false},
		
		// Ratings indexes
		{"idx_ratings_ride_id", "ratings", "ride_id", true},
		{"idx_ratings_driver_id", "ratings", "driver_id", false},
		{"idx_ratings_rider_id", "ratings", "rider_id", false},
		
		// OTP indexes
		{"idx_otps_phone", "otps", "phone", false},
		{"idx_otps_code", "otps", "code", false},
		{"idx_otps_expires", "otps", "expires_at", false},
		
		// Wallet indexes
		{"idx_wallets_user_id", "wallets", "user_id", true},
		
		// Device tokens indexes
		{"idx_device_tokens_user_id", "device_tokens", "user_id", false},
		{"idx_device_tokens_token", "device_tokens", "token", true},
	}
	
	for _, idx := range indexes {
		unique := ""
		if idx.Unique {
			unique = "UNIQUE "
		}
		
		sql := fmt.Sprintf("CREATE %sINDEX IF NOT EXISTS %s ON %s (%s)", 
			unique, idx.Name, idx.Table, idx.Columns)
		
		if err := db.Exec(sql).Error; err != nil {
			utils.Error("Failed to create index", 
				zap.String("index", idx.Name),
				zap.Error(err))
			// Continue with other indexes
			continue
		}
		
		utils.Info("Created index", zap.String("name", idx.Name))
	}
	
	return nil
}

// CreatePartitions creates table partitions for rides (monthly)
func CreateRidePartitions(db *gorm.DB) error {
	utils.Info("Creating ride table partitions...")
	
	// This is for PostgreSQL
	// Partition rides table by month for better query performance
	sql := `
		CREATE TABLE IF NOT EXISTS rides_partitioned (
			LIKE rides INCLUDING ALL
		) PARTITION BY RANGE (created_at);
	`
	
	if err := db.Exec(sql).Error; err != nil {
		return fmt.Errorf("failed to create partitioned table: %w", err)
	}
	
	return nil
}

// AnalyzeTables runs ANALYZE on all tables for query planner
func AnalyzeTables(db *gorm.DB) error {
	tables := []string{
		"rides", "drivers", "users", "payments", "transactions",
		"driver_locations", "ride_locations", "ratings", "notifications",
	}
	
	for _, table := range tables {
		sql := fmt.Sprintf("ANALYZE %s", table)
		if err := db.Exec(sql).Error; err != nil {
			utils.Error("Failed to analyze table", 
				zap.String("table", table),
				zap.Error(err))
		}
	}
	
	return nil
}
