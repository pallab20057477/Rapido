package main

import (
	"fmt"
	"log"

	"rapido-backend/config"
	"rapido-backend/database"
	"rapido-backend/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func setupRider() {
	// Initialize config and database
	cfg := config.Load()
	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	database.DB = db

	// Create rider user
	riderPhone := "9876543210"
	riderEmail := "rider@example.com"
	riderName := "Test Rider"

	fmt.Printf("Setting up rider user...\n")

	var existingUser models.User
	err = database.DB.Where("phone = ?", riderPhone).First(&existingUser).Error

	if err == nil {
		fmt.Printf("User with phone %s already exists:\n", riderPhone)
		fmt.Printf("ID: %s\n", existingUser.ID)
		fmt.Printf("Name: %s\n", existingUser.Name)
		fmt.Printf("Role: %s\n", existingUser.Role)
		fmt.Printf("IsActive: %t\n", existingUser.IsActive)

		// Update to ensure it's a rider and active
		if existingUser.Role != "rider" || !existingUser.IsActive {
			fmt.Printf("Updating user to be an active rider...\n")
			updates := map[string]interface{}{
				"role":      "rider",
				"is_active": true,
				"name":      riderName,
				"email":     riderEmail,
			}
			if err := database.DB.Model(&existingUser).Updates(updates).Error; err != nil {
				log.Fatalf("Failed to update user: %v", err)
			}
			fmt.Printf("Successfully updated user role to rider and set active\n")
		} else {
			fmt.Printf("User is already an active rider\n")
		}

		fmt.Printf("\nRider ready for testing!\n")
		fmt.Printf("Phone: %s\n", existingUser.Phone)
		fmt.Printf("User ID: %s\n", existingUser.ID)
		fmt.Printf("Use this phone number for OTP login in Postman\n")

	} else if err == gorm.ErrRecordNotFound {
		// Create new rider user
		fmt.Printf("Creating new rider user...\n")
		newRider := models.User{
			ID:       uuid.New(),
			Name:     riderName,
			Email:    riderEmail,
			Phone:    riderPhone,
			Role:     "rider",
			IsActive: true,
			Provider: "local",
		}

		if err := database.DB.Create(&newRider).Error; err != nil {
			log.Fatalf("Failed to create rider: %v", err)
		}

		fmt.Printf("Successfully created rider user:\n")
		fmt.Printf("ID: %s\n", newRider.ID)
		fmt.Printf("Name: %s\n", newRider.Name)
		fmt.Printf("Phone: %s\n", newRider.Phone)
		fmt.Printf("Role: %s\n", newRider.Role)
		fmt.Printf("IsActive: %t\n", newRider.IsActive)

		fmt.Printf("\nRider ready for testing!\n")
		fmt.Printf("Use phone %s for OTP login in Postman\n", riderPhone)

	} else {
		log.Fatalf("Database error: %v", err)
	}

	// Also verify the driver user exists
	driverPhone := "9876543211"
	var driver models.User
	if err := database.DB.Where("phone = ?", driverPhone).First(&driver).Error; err == nil {
		fmt.Printf("\nDriver user verification:\n")
		fmt.Printf("ID: %s\n", driver.ID)
		fmt.Printf("Name: %s\n", driver.Name)
		fmt.Printf("Phone: %s\n", driver.Phone)
		fmt.Printf("Role: %s\n", driver.Role)
		fmt.Printf("IsActive: %t\n", driver.IsActive)
	}

	fmt.Printf("\nSetup complete! Now you can:\n")
	fmt.Printf("1. Use Postman to request OTP for rider: %s\n", riderPhone)
	fmt.Printf("2. Verify OTP and get rider access token\n")
	fmt.Printf("3. Use the rider token to request rides\n")
}
