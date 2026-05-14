package services

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"rapido-backend/config"
	"rapido-backend/database"
	"rapido-backend/models"
	"rapido-backend/utils"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// EnsureAdminUser creates or refreshes the configured admin account.
func EnsureAdminUser(cfg *config.Config) error {
	if cfg == nil {
		cfg = config.Get()
	}

	if database.DB == nil {
		return errors.New("database is not initialized")
	}

	email := strings.ToLower(strings.TrimSpace(cfg.Admin.Email))
	password := strings.TrimSpace(cfg.Admin.Password)
	name := strings.TrimSpace(cfg.Admin.Name)
	phone := strings.TrimSpace(cfg.Admin.Phone)

	if email == "" || password == "" || name == "" || phone == "" {
		utils.Warn("Admin credentials are not fully configured; skipping bootstrap")
		return nil
	}

	hashedPassword, err := HashPassword(password)
	if err != nil {
		return fmt.Errorf("failed to hash admin password: %w", err)
	}

	return database.DB.Transaction(func(tx *gorm.DB) error {
		var user models.User

		if err := tx.Where("LOWER(email) = LOWER(?)", email).First(&user).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("failed to look up admin user by email: %w", err)
			}

			if err := tx.Where("phone = ?", phone).First(&user).Error; err != nil {
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					return fmt.Errorf("failed to look up admin user by phone: %w", err)
				}

				user = models.User{
					Email:         email,
					Phone:         phone,
					Name:          name,
					Role:          models.AdminRoleAdmin,
					Provider:      "local",
					EmailVerified: true,
					IsActive:      true,
					PasswordHash:  hashedPassword,
				}

				if err := tx.Create(&user).Error; err != nil {
					return fmt.Errorf("failed to create admin user: %w", err)
				}

				log.Printf("[AdminBootstrap] Created admin user %s", email)
				return nil
			}

			if user.Email != "" && !strings.EqualFold(user.Email, email) {
				return fmt.Errorf("admin phone %s is already linked to another account", phone)
			}
		} else {
			var phoneOwner models.User
			if err := tx.Where("phone = ? AND id <> ?", phone, user.ID).First(&phoneOwner).Error; err == nil {
				return fmt.Errorf("admin phone %s is already linked to another account", phone)
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("failed to check admin phone ownership: %w", err)
			}
		}

		updates := map[string]interface{}{
			"name":           name,
			"phone":          phone,
			"role":           models.AdminRoleAdmin,
			"provider":       "local",
			"email_verified": true,
			"is_active":      true,
			"password_hash":  hashedPassword,
		}

		if err := tx.Model(&user).Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to update admin user: %w", err)
		}

		utils.Info("Admin user updated",
			zap.String("email", email))
		return nil
	})
}
