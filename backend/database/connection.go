package database

import (
	"fmt"
	"log"
	"os"
	"strings"

	"rapido-backend/config"
	"rapido-backend/models"
	"rapido-backend/utils"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Connect(cfg *config.Config) (*gorm.DB, error) {
	// First, try to create database if it doesn't exist
	if err := createDatabaseIfNotExists(cfg); err != nil {
		utils.Warn("Could not auto-create database", zap.Error(err))
	}

	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Name,
		cfg.Database.SSLMode,
	)

	gormConfig := &gorm.Config{
		Logger:                                   logger.Default.LogMode(logger.Silent), // Silent by default - only show errors
		DisableForeignKeyConstraintWhenMigrating: true,
	}

	// Enable SQL logging if explicitly configured via DB_LOG_LEVEL or legacy debug flag
	if strings.ToLower(os.Getenv("DB_LOG_LEVEL")) == "debug" || config.Get().Server.Mode == "debug_sql" {
		gormConfig.Logger = logger.Default.LogMode(logger.Info)
	}

	db, err := gorm.Open(postgres.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	DB = db
	return db, nil
}

// createDatabaseIfNotExists connects to the default 'postgres' database and creates the target database if it doesn't exist
func createDatabaseIfNotExists(cfg *config.Config) error {
	// Connect to default 'postgres' database to create new database
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=postgres sslmode=%s",
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.SSLMode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return fmt.Errorf("failed to connect to postgres database: %w", err)
	}

	// Check if database exists
	var count int64
	result := db.Raw("SELECT COUNT(*) FROM pg_database WHERE datname = ?", cfg.Database.Name).Scan(&count)
	if result.Error != nil {
		return fmt.Errorf("failed to check database existence: %w", result.Error)
	}

	// Create database if it doesn't exist
	if count == 0 {
		utils.Info("Database does not exist, creating...",
			zap.String("database", cfg.Database.Name))
		createSQL := fmt.Sprintf("CREATE DATABASE \"%s\"", cfg.Database.Name)
		if result := db.Exec(createSQL); result.Error != nil {
			return fmt.Errorf("failed to create database: %w", result.Error)
		}
		utils.Info("Database created successfully",
			zap.String("database", cfg.Database.Name))
	}

	sqlDB, _ := db.DB()
	sqlDB.Close()
	return nil
}

func Migrate(db *gorm.DB) error {
	utils.Info("Running database migrations...")

	if err := ensureRequiredExtensions(db); err != nil {
		return err
	}

	if err := migratePublicUsersTable(db); err != nil {
		return err
	}

	models := []interface{}{
		&models.EmergencyContact{},
		&models.SOSEvent{},
		&models.SOSNotification{},
		&models.AuditLog{},
		&models.SupportTicket{},
		&models.SupportTicketMessage{},
		&models.Dispute{},
		&models.PaymentMethod{},
		&models.UPIDetails{},
		&models.Device{},
		&models.DeviceSession{},
		&models.OTP{},
		&models.RefreshToken{},
		&models.Driver{},
		&models.DriverLocation{},
		&models.Vehicle{},
		&models.DriverDocument{},
		&models.DriverEarnings{},
		&models.DriverStatusLog{},
		&models.Ride{},
		&models.RideLocation{},
		&models.RideRequestLog{},
		&models.RideStatusLog{},
		&models.RideMatch{},
		&models.SurgePricing{},
		&models.FareConfig{},
		&models.Incentive{},
		&models.DriverIncentive{},
		&models.WeeklyTarget{},
		&models.Payment{},
		&models.Transaction{},
		&models.Wallet{},
		&models.LedgerAccount{},
		&models.LedgerEntry{},
		&models.Commission{},
		&models.Withdrawal{},
		&models.Invoice{},
		&models.Rating{},
		&models.DriverRatingSummary{},
		&models.SOSAlert{},
		&models.TripShare{},
		&models.ShareRecipient{},
		&models.SafetyCheckIn{},
		&models.SafetySettings{},
		&models.IncidentReport{},
		&models.Notification{},
		&models.NotificationPreference{},
		&models.DeviceToken{},
		&models.NotificationQueue{},
		&models.ChatRoom{},
		&models.ChatMessage{},
		&models.ChatReadReceipt{},
		&models.ChatQuickReply{},
		&models.Admin{},
		&models.AdminActivityLog{},
		&models.SystemSettings{},
		&models.PromoCode{},
		&models.PromoCodeUsage{},
		&models.City{},
	}

	for _, model := range models {
		if err := db.AutoMigrate(model); err != nil {
			// Some existing databases may contain legacy types or expressions that
			// cause GORM's introspection queries to fail (e.g. malformed column
			// defaults). Rather than aborting the entire migration process for
			// those cases, log and skip the problematic model so remaining
			// migrations can proceed. If the error is unexpected, fail fast.
			errStr := err.Error()
			if strings.Contains(errStr, "insufficient arguments") || strings.Contains(errStr, "malformed") {
				utils.Warn("Skipping migration due to introspection error",
					zap.String("model", fmt.Sprintf("%T", model)),
					zap.Error(err))
				continue
			}
			return fmt.Errorf("failed to migrate %T: %w", model, err)
		}
	}

	if err := fixEmergencyContactForeignKey(db); err != nil {
		return err
	}

	if err := fixRidesForeignKey(db); err != nil {
		return err
	}

	utils.Info("Database migrations completed")
	return seedInitialData(db)
}

func ensureRequiredExtensions(db *gorm.DB) error {
	if err := db.Exec(`CREATE EXTENSION IF NOT EXISTS pgcrypto`).Error; err != nil {
		return fmt.Errorf("failed to enable pgcrypto extension: %w", err)
	}

	return nil
}

func migratePublicUsersTable(db *gorm.DB) error {
	if db.Migrator().HasTable("public_users") {
		return nil
	}

	if db.Migrator().HasTable("app_users") {
		log.Println("Renaming legacy app_users table to public_users...")
		if err := db.Exec(`ALTER TABLE app_users RENAME TO public_users`).Error; err != nil {
			return fmt.Errorf("failed to rename app_users to public_users: %w", err)
		}
		return nil
	}

	log.Println("Creating public_users table...")
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS public_users (
			id uuid PRIMARY KEY,
			name text NOT NULL DEFAULT '',
			email text NOT NULL DEFAULT '',
			phone text NOT NULL DEFAULT '',
			password_hash text NOT NULL DEFAULT '',
			provider text NOT NULL DEFAULT '',
			provider_id text NOT NULL DEFAULT '',
			email_verified boolean NOT NULL DEFAULT false,
			profile_image text NOT NULL DEFAULT '',
			role text NOT NULL DEFAULT 'rider',
			google_id text NOT NULL DEFAULT '',
			is_active boolean NOT NULL DEFAULT true,
			latitude double precision NOT NULL DEFAULT 0,
			longitude double precision NOT NULL DEFAULT 0,
			address text NOT NULL DEFAULT '',
			location_updated_at timestamptz,
			created_at timestamptz NOT NULL DEFAULT NOW(),
			updated_at timestamptz NOT NULL DEFAULT NOW()
		)
	`).Error; err != nil {
		return fmt.Errorf("failed to create public_users table: %w", err)
	}

	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_public_users_email ON public_users (LOWER(email))`).Error; err != nil {
		return fmt.Errorf("failed to create public_users email index: %w", err)
	}

	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_public_users_phone ON public_users (phone)`).Error; err != nil {
		return fmt.Errorf("failed to create public_users phone index: %w", err)
	}

	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_public_users_google_id ON public_users (google_id)`).Error; err != nil {
		return fmt.Errorf("failed to create public_users google_id index: %w", err)
	}

	return nil
}

func fixEmergencyContactForeignKey(db *gorm.DB) error {
	log.Println("Ensuring emergency_contacts.user_id references public_users.id...")

	// The auth/profile flow uses PublicUser -> public_users, while an older FK
	// still points emergency_contacts at users. That causes valid users to fail
	// on insert even though profile lookup succeeds.
	var constraintNames []string
	if err := db.Raw(`
		SELECT tc.constraint_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		WHERE tc.table_name = 'emergency_contacts'
			AND tc.constraint_type = 'FOREIGN KEY'
			AND kcu.column_name = 'user_id'
	`).Scan(&constraintNames).Error; err != nil {
		return fmt.Errorf("failed to inspect emergency_contacts foreign keys: %w", err)
	}

	for _, constraintName := range constraintNames {
		if err := db.Exec(fmt.Sprintf(`ALTER TABLE emergency_contacts DROP CONSTRAINT IF EXISTS "%s"`, strings.ReplaceAll(constraintName, `"`, `""`))).Error; err != nil {
			return fmt.Errorf("failed to drop stale emergency_contacts foreign key %s: %w", constraintName, err)
		}
	}

	if err := db.Exec(`
		ALTER TABLE emergency_contacts
		ADD CONSTRAINT fk_users_emergency_contacts
		FOREIGN KEY (user_id) REFERENCES public_users(id)
		ON UPDATE CASCADE ON DELETE CASCADE
	`).Error; err != nil {
		return fmt.Errorf("failed to create emergency_contacts foreign key to public_users: %w", err)
	}

	return nil
}

func fixRidesForeignKey(db *gorm.DB) error {
	log.Println("Fixing foreign keys that should reference public_users...")

	// Multiple tables reference User (PublicUser -> public_users), but older FK constraints
	// may still point to a non-existent 'users' table. This causes failures on insert.
	// Tables that need fixing: rides (rider_id), drivers (user_id), admin (user_id)

	tablesToFix := []struct {
		tableName  string
		columnName string
	}{
		{"rides", "rider_id"},
		{"drivers", "user_id"},
		{"admin", "user_id"},
	}

	for _, tf := range tablesToFix {
		// Skip if table doesn't exist yet (it will be created by AutoMigrate)
		if !db.Migrator().HasTable(tf.tableName) {
			log.Printf("Table %s does not exist yet (will be created), skipping FK fix", tf.tableName)
			continue
		}

		log.Printf("Fixing %s.%s to reference public_users...", tf.tableName, tf.columnName)

		var constraintNames []string
		if err := db.Raw(fmt.Sprintf(`
			SELECT tc.constraint_name
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage kcu
				ON tc.constraint_name = kcu.constraint_name
				AND tc.table_schema = kcu.table_schema
			WHERE tc.table_name = '%s'
				AND tc.constraint_type = 'FOREIGN KEY'
				AND kcu.column_name = '%s'
		`, tf.tableName, tf.columnName)).Scan(&constraintNames).Error; err != nil {
			log.Printf("Warning: failed to inspect %s foreign keys: %v", tf.tableName, err)
			continue
		}

		// Drop all existing FK constraints on the column
		for _, constraintName := range constraintNames {
			if err := db.Exec(fmt.Sprintf(`ALTER TABLE %s DROP CONSTRAINT IF EXISTS "%s"`,
				tf.tableName, strings.ReplaceAll(constraintName, `"`, `""`))).Error; err != nil {
				log.Printf("Warning: failed to drop stale %s foreign key %s: %v", tf.tableName, constraintName, err)
				continue
			}
		}

		// Create correct FK constraint pointing to public_users
		fkName := fmt.Sprintf("fk_%s_%s", tf.tableName, tf.columnName)
		if err := db.Exec(fmt.Sprintf(`
			ALTER TABLE %s
			ADD CONSTRAINT %s
			FOREIGN KEY (%s) REFERENCES public_users(id)
			ON UPDATE CASCADE ON DELETE CASCADE
		`, tf.tableName, fkName, tf.columnName)).Error; err != nil {
			log.Printf("Warning: failed to create %s foreign key: %v", tf.tableName, err)
			continue
		}

		log.Printf("Fixed %s.%s foreign key to reference public_users", tf.tableName, tf.columnName)
	}

	return nil
}

func seedInitialData(db *gorm.DB) error {
	// Seed fare configurations
	fareConfigs := []models.FareConfig{
		{
			VehicleType:     models.VehicleTypeBike,
			BaseFare:        30,
			PerKmRate:       8,
			PerMinRate:      1,
			MinFare:         30,
			MaxFare:         500,
			PlatformFee:     5,
			ServiceFee:      2,
			NightMultiplier: 1.25,
			IsActive:        true,
		},
		{
			VehicleType:     models.VehicleTypeAuto,
			BaseFare:        40,
			PerKmRate:       12,
			PerMinRate:      1.5,
			MinFare:         40,
			MaxFare:         800,
			PlatformFee:     8,
			ServiceFee:      3,
			NightMultiplier: 1.25,
			IsActive:        true,
		},
		{
			VehicleType:     models.VehicleTypeCarGo,
			BaseFare:        60,
			PerKmRate:       15,
			PerMinRate:      2,
			MinFare:         60,
			MaxFare:         1500,
			PlatformFee:     10,
			ServiceFee:      5,
			NightMultiplier: 1.25,
			IsActive:        true,
		},
		{
			VehicleType:     models.VehicleTypeCarX,
			BaseFare:        80,
			PerKmRate:       20,
			PerMinRate:      3,
			MinFare:         80,
			MaxFare:         2000,
			PlatformFee:     15,
			ServiceFee:      8,
			NightMultiplier: 1.3,
			IsActive:        true,
		},
	}

	for _, fare := range fareConfigs {
		var existing models.FareConfig
		if err := db.Where("vehicle_type = ?", fare.VehicleType).First(&existing).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				if err := db.Create(&fare).Error; err != nil {
					return err
				}
			}
		}
	}

	// Seed quick replies
	quickReplies := []models.ChatQuickReply{
		{Category: "both", Message: "I'm here", Language: "en", Order: 1},
		{Category: "rider", Message: "I'm running late", Language: "en", Order: 2},
		{Category: "driver", Message: "I'll be there in 2 minutes", Language: "en", Order: 2},
		{Category: "rider", Message: "Where are you?", Language: "en", Order: 3},
		{Category: "driver", Message: "I'm at the pickup location", Language: "en", Order: 3},
		{Category: "both", Message: "Thank you!", Language: "en", Order: 4},
		{Category: "rider", Message: "Please wait, I'm coming", Language: "en", Order: 5},
	}

	for _, reply := range quickReplies {
		var existing models.ChatQuickReply
		if err := db.Where("message = ? AND category = ?", reply.Message, reply.Category).First(&existing).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				if err := db.Create(&reply).Error; err != nil {
					return err
				}
			}
		}
	}

	return nil
}
