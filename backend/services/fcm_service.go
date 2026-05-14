package services

import (
	"context"
	"fmt"
	"log"

	"rapido-backend/config"
	"rapido-backend/database"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"github.com/google/uuid"
	"google.golang.org/api/option"
)

// FCMService handles Firebase Cloud Messaging push notifications
type FCMService struct {
	client      *messaging.Client
	firebaseApp *firebase.App
	isEnabled   bool
}

// FCMServiceInstance global instance
var FCMServiceInstance *FCMService

// InitFCMService initializes the FCM service with Firebase credentials
func InitFCMService() error {
	cfg := config.Get()
	if cfg.FCM.CredentialsFile == "" {
		log.Println("[FCM] No Firebase credentials file configured, FCM disabled")
		FCMServiceInstance = &FCMService{isEnabled: false}
		return nil
	}

	ctx := context.Background()
	opt := option.WithCredentialsFile(cfg.FCM.CredentialsFile)
	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		return fmt.Errorf("failed to initialize Firebase app: %w", err)
	}

	client, err := app.Messaging(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize Firebase Messaging: %w", err)
	}

	FCMServiceInstance = &FCMService{
		client:      client,
		firebaseApp: app,
		isEnabled:   true,
	}
	log.Println("[FCM] Firebase Cloud Messaging initialized successfully")
	return nil
}

// GetFCMService returns the global FCM service instance
func GetFCMService() *FCMService {
	return FCMServiceInstance
}

// IsEnabled returns whether FCM is configured and available
func (f *FCMService) IsEnabled() bool {
	return f.isEnabled && f.client != nil
}

// SendPushNotification sends a generic push notification to a user
func (f *FCMService) SendPushNotification(userID interface{}, title, body string, data map[string]interface{}) error {
	if !f.IsEnabled() {
		log.Printf("[FCM] Push notification (disabled): %s - %s", title, body)
		return nil
	}

	uid, err := parseUserID(userID)
	if err != nil {
		return err
	}

	// Get user's FCM token from database
	token, err := f.getUserFCMToken(uid)
	if err != nil {
		return fmt.Errorf("failed to get FCM token for user %s: %w", uid, err)
	}
	if token == "" {
		log.Printf("[FCM] No FCM token for user %s, skipping push notification", uid)
		return nil
	}

	// Convert data to map[string]string for FCM
	stringData := make(map[string]string)
	for k, v := range data {
		if str, ok := v.(string); ok {
			stringData[k] = str
		} else {
			stringData[k] = fmt.Sprintf("%v", v)
		}
	}

	message := &messaging.Message{
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data:  stringData,
		Token: token,
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				ChannelID: "rapido_notifications",
				Priority:  messaging.PriorityHigh,
			},
		},
		APNS: &messaging.APNSConfig{
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Alert: &messaging.ApsAlert{
						Title: title,
						Body:  body,
					},
					Badge: func() *int { i := 1; return &i }(),
					Sound: "default",
				},
			},
		},
	}

	ctx := context.Background()
	_, err = f.client.Send(ctx, message)
	if err != nil {
		// Check if token is invalid
		if messaging.IsRegistrationTokenNotRegistered(err) {
			f.invalidateUserFCMToken(uid)
		}
		return fmt.Errorf("failed to send FCM message: %w", err)
	}

	log.Printf("[FCM] Push notification sent to user %s: %s", uid, title)
	return nil
}

// SendRideRequestToDriver sends a ride request notification to a driver
func (f *FCMService) SendRideRequestToDriver(driverID interface{}, rideID, pickup string, distanceKm float64, eta int, estimatedFare float64) error {
	data := map[string]interface{}{
		"type":           "ride_request",
		"ride_id":        rideID,
		"pickup":         pickup,
		"distance_km":    fmt.Sprintf("%.2f", distanceKm),
		"eta":            fmt.Sprintf("%d", eta),
		"estimated_fare": fmt.Sprintf("%.2f", estimatedFare),
	}

	return f.SendPushNotification(driverID, "New Ride Request", fmt.Sprintf("Pickup at %s (%.1f km, %d min)", pickup, distanceKm, eta), data)
}

// SendDriverAssignedToRider notifies rider that a driver has been assigned
func (f *FCMService) SendDriverAssignedToRider(riderID, driverID interface{}, rideID, driverName, vehicleNumber string, eta int) error {
	data := map[string]interface{}{
		"type":           "driver_assigned",
		"ride_id":        rideID,
		"driver_id":      fmt.Sprintf("%v", driverID),
		"driver_name":    driverName,
		"vehicle_number": vehicleNumber,
		"eta":            fmt.Sprintf("%d", eta),
	}

	return f.SendPushNotification(riderID, "Driver Assigned", fmt.Sprintf("%s is arriving in %d min (%s)", driverName, eta, vehicleNumber), data)
}

// SendDriverArrivedToRider notifies rider that driver has arrived at pickup
func (f *FCMService) SendDriverArrivedToRider(riderID interface{}, rideID, driverName string) error {
	data := map[string]interface{}{
		"type":    "driver_arrived",
		"ride_id": rideID,
	}

	return f.SendPushNotification(riderID, "Driver Arrived", fmt.Sprintf("%s has arrived at your pickup location", driverName), data)
}

// SendRideStartedToRider notifies rider that ride has started
func (f *FCMService) SendRideStartedToRider(riderID interface{}, rideID string) error {
	data := map[string]interface{}{
		"type":    "ride_started",
		"ride_id": rideID,
	}

	return f.SendPushNotification(riderID, "Ride Started", "Your ride has started. Enjoy your journey!", data)
}

// SendRideCompletedToRider notifies rider that ride is complete
func (f *FCMService) SendRideCompletedToRider(riderID interface{}, rideID string, finalFare float64) error {
	data := map[string]interface{}{
		"type":       "ride_completed",
		"ride_id":    rideID,
		"final_fare": fmt.Sprintf("%.2f", finalFare),
	}

	return f.SendPushNotification(riderID, "Ride Completed", fmt.Sprintf("Ride completed. Fare: ₹%.2f", finalFare), data)
}

// SendPaymentFailedToRider notifies rider of payment failure
func (f *FCMService) SendPaymentFailedToRider(riderID interface{}, rideID string, amount float64, reason string) error {
	data := map[string]interface{}{
		"type":    "payment_failed",
		"ride_id": rideID,
		"amount":  fmt.Sprintf("%.2f", amount),
		"reason":  reason,
	}

	return f.SendPushNotification(riderID, "Payment Failed", fmt.Sprintf("Payment of ₹%.2f failed: %s. Please retry.", amount, reason), data)
}

// SendSOSTriggered sends emergency alert to safety team
func (f *FCMService) SendSOSTriggered(userID interface{}, rideID, location string) error {
	data := map[string]interface{}{
		"type":     "sos_triggered",
		"ride_id":  rideID,
		"location": location,
		"user_id":  fmt.Sprintf("%v", userID),
	}

	// Send to safety topic instead of individual user
	return f.sendToTopic("safety_alerts", "🚨 SOS Alert", fmt.Sprintf("Emergency triggered for ride %s at %s", rideID, location), data)
}

// SubscribeToTopic subscribes tokens to a topic
func (f *FCMService) SubscribeToTopic(tokens []string, topic string) error {
	if !f.IsEnabled() {
		return nil
	}

	ctx := context.Background()
	resp, err := f.client.SubscribeToTopic(ctx, tokens, topic)
	if err != nil {
		return fmt.Errorf("failed to subscribe to topic: %w", err)
	}

	if len(resp.Errors) > 0 {
		log.Printf("[FCM] Subscribe errors: %v", resp.Errors)
	}

	log.Printf("[FCM] Subscribed %d tokens to topic %s", len(tokens), topic)
	return nil
}

// UnsubscribeFromTopic unsubscribes tokens from a topic
func (f *FCMService) UnsubscribeFromTopic(tokens []string, topic string) error {
	if !f.IsEnabled() {
		return nil
	}

	ctx := context.Background()
	resp, err := f.client.UnsubscribeFromTopic(ctx, tokens, topic)
	if err != nil {
		return fmt.Errorf("failed to unsubscribe from topic: %w", err)
	}

	if len(resp.Errors) > 0 {
		log.Printf("[FCM] Unsubscribe errors: %v", resp.Errors)
	}

	log.Printf("[FCM] Unsubscribed %d tokens from topic %s", len(tokens), topic)
	return nil
}

// sendToTopic sends a message to a topic
func (f *FCMService) sendToTopic(topic, title, body string, data map[string]interface{}) error {
	if !f.IsEnabled() {
		log.Printf("[FCM] Topic message (disabled): %s - %s", title, body)
		return nil
	}

	// Convert data to map[string]string for FCM
	stringData := make(map[string]string)
	for k, v := range data {
		if str, ok := v.(string); ok {
			stringData[k] = str
		} else {
			stringData[k] = fmt.Sprintf("%v", v)
		}
	}

	message := &messaging.Message{
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data:  stringData,
		Topic: topic,
		Android: &messaging.AndroidConfig{
			Priority: "high",
		},
		APNS: &messaging.APNSConfig{
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Alert: &messaging.ApsAlert{
						Title: title,
						Body:  body,
					},
					Badge: func() *int { i := 1; return &i }(),
					Sound: "default",
				},
			},
		},
	}

	ctx := context.Background()
	_, err := f.client.Send(ctx, message)
	if err != nil {
		return fmt.Errorf("failed to send topic message: %w", err)
	}

	log.Printf("[FCM] Topic message sent to %s: %s", topic, title)
	return nil
}

// getUserFCMToken retrieves the FCM token for a user from Redis/DB
func (f *FCMService) getUserFCMToken(userID uuid.UUID) (string, error) {
	// Try Redis first
	token, err := database.GetCache(fmt.Sprintf("fcm_token:%s", userID))
	if err == nil && token != "" {
		return token, nil
	}

	// Could fetch from database if not in cache
	// For now, return empty - caller should handle missing token
	return "", nil
}

// invalidateUserFCMToken invalidates a user's FCM token
func (f *FCMService) invalidateUserFCMToken(userID uuid.UUID) {
	key := fmt.Sprintf("fcm_token:%s", userID)
	database.DeleteCache(key)
	log.Printf("[FCM] Invalidated FCM token for user %s", userID)
}

// parseUserID converts various user ID types to uuid.UUID
func parseUserID(userID interface{}) (uuid.UUID, error) {
	switch v := userID.(type) {
	case uuid.UUID:
		return v, nil
	case string:
		return uuid.Parse(v)
	case []byte:
		return uuid.Parse(string(v))
	default:
		return uuid.Parse(fmt.Sprintf("%v", userID))
	}
}

// StoreFCMToken stores a user's FCM token
func (f *FCMService) StoreFCMToken(userID uuid.UUID, token string) error {
	key := fmt.Sprintf("fcm_token:%s", userID)
	err := database.SetCache(key, token, 2592000) // 30 days expiry
	if err != nil {
		return fmt.Errorf("failed to store FCM token: %w", err)
	}
	log.Printf("[FCM] Stored FCM token for user %s", userID)
	return nil
}

// SendMulticast sends a message to multiple tokens
func (f *FCMService) SendMulticast(tokens []string, title, body string, data map[string]string) (*messaging.BatchResponse, error) {
	if !f.IsEnabled() {
		return nil, nil
	}

	message := &messaging.MulticastMessage{
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data:   data,
		Tokens: tokens,
		Android: &messaging.AndroidConfig{
			Priority: "high",
		},
	}

	ctx := context.Background()
	resp, err := f.client.SendMulticast(ctx, message)
	if err != nil {
		return nil, fmt.Errorf("failed to send multicast: %w", err)
	}

	// Handle invalid tokens
	for i, res := range resp.Responses {
		if res.Error != nil && messaging.IsRegistrationTokenNotRegistered(res.Error) {
			// Token is invalid, remove it from database
			log.Printf("[FCM] Invalid token detected: %s", tokens[i])
		}
	}

	return resp, nil
}
