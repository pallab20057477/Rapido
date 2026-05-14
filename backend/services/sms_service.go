package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"rapido-backend/config"
	"rapido-backend/utils"

	"go.uber.org/zap"
)

// SMSService handles SMS notifications for fallback and critical alerts
type SMSService struct {
	twilioConfig config.TwilioConfig
	msg91Config  MSG91Config
	provider     string
}

// MSG91Config holds MSG91 (Indian SMS provider) credentials
type MSG91Config struct {
	AuthKey  string
	SenderID string
	Route    string
}

// NewSMSService creates a new SMS service
func NewSMSService() *SMSService {
	cfg := config.Get()
	
	// Determine which provider to use
	provider := "twilio"
	if cfg.Twilio.AccountSID == "" {
		provider = "msg91" // Fallback to Indian provider
	}
	
	return &SMSService{
		twilioConfig: cfg.Twilio,
		msg91Config: MSG91Config{
			AuthKey:  utils.GetEnv("MSG91_AUTH_KEY", ""),
			SenderID: utils.GetEnv("MSG91_SENDER_ID", "RAPIDO"),
			Route:    utils.GetEnv("MSG91_ROUTE", "4"), // 4 = transactional
		},
		provider: provider,
	}
}

// SendSMS sends an SMS to a phone number
func (s *SMSService) SendSMS(phone, message string) error {
	if s.provider == "twilio" && s.twilioConfig.AccountSID != "" {
		return s.sendViaTwilio(phone, message)
	}
	
	if s.msg91Config.AuthKey != "" {
		return s.sendViaMSG91(phone, message)
	}
	
	return fmt.Errorf("no SMS provider configured")
}

// SendOTP sends an OTP SMS
func (s *SMSService) SendOTP(phone, otp string) error {
	message := fmt.Sprintf("Your Rapido OTP is %s. Valid for 5 minutes. Do not share this code.", otp)
	return s.SendSMS(phone, message)
}

// SendDriverArrivalNotification notifies rider that driver has arrived
func (s *SMSService) SendDriverArrivalNotification(phone, driverName string) error {
	message := fmt.Sprintf("Your Rapido driver %s has arrived at your pickup location. Please meet them.", driverName)
	return s.SendSMS(phone, message)
}

// SendRideStartConfirmation confirms ride has started
func (s *SMSService) SendRideStartConfirmation(phone string) error {
	message := "Your Rapido ride has started. Enjoy your journey! Track your ride in the app."
	return s.SendSMS(phone, message)
}

// SendRideCompleteNotification notifies rider of completed ride
func (s *SMSService) SendRideCompleteNotification(phone string, finalFare float64) error {
	message := fmt.Sprintf("Your Rapido ride is complete. Fare: Rs.%.2f. Thank you for riding with us!", finalFare)
	return s.SendSMS(phone, message)
}

// SendPaymentFailureAlert alerts rider of payment failure
func (s *SMSService) SendPaymentFailureAlert(phone string, amount float64) error {
	message := fmt.Sprintf("Payment of Rs.%.2f failed. Please retry or check your payment method in the app.", amount)
	return s.SendSMS(phone, message)
}

// SendSOSAlertToEmergencyContact sends SOS to emergency contact
func (s *SMSService) SendSOSAlertToEmergencyContact(contactPhone, riderName, location string) error {
	message := fmt.Sprintf("🆘 EMERGENCY ALERT: %s triggered SOS. Last known location: %s. Contact immediately!", riderName, location)
	return s.SendSMS(contactPhone, message)
}

// sendViaTwilio sends SMS using Twilio
func (s *SMSService) sendViaTwilio(phone, message string) error {
	// Format phone number — country code from env, default +91 (India)
	countryCode := utils.GetEnv("SMS_COUNTRY_CODE", "+91")
	if !strings.HasPrefix(phone, "+") {
		phone = countryCode + phone
	}
	
	// Twilio API endpoint
	urlStr := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", s.twilioConfig.AccountSID)
	
	// Build request body
	msgData := url.Values{}
	msgData.Set("To", phone)
	msgData.Set("From", s.twilioConfig.PhoneNumber)
	msgData.Set("Body", message)
	msgDataReader := strings.NewReader(msgData.Encode())
	
	// Create request
	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("POST", urlStr, msgDataReader)
	req.SetBasicAuth(s.twilioConfig.AccountSID, s.twilioConfig.AuthToken)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	
	// Make request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send Twilio SMS: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode >= 400 {
		return fmt.Errorf("twilio API error: %d", resp.StatusCode)
	}
	
	utils.Info("SMS sent via Twilio", zap.String("phone", phone[:6]+"****"))
	return nil
}

// sendViaMSG91 sends SMS using MSG91 (Indian provider)
func (s *SMSService) sendViaMSG91(phone, message string) error {
	// Strip country code prefix if present
	countryCode := strings.TrimPrefix(utils.GetEnv("SMS_COUNTRY_CODE", "+91"), "+")
	phone = strings.TrimPrefix(phone, "+"+countryCode)
	phone = strings.TrimPrefix(phone, countryCode)
	
	// MSG91 API endpoint
	url := "https://api.msg91.com/api/v5/flow/"
	
	// Build payload
	payload := map[string]interface{}{
		"template_id": s.getMSG91TemplateID(message),
		"short_url":   "0",
		"recipients": []map[string]string{
			{
				"mobiles": phone,
				"message": message,
			},
		},
	}
	
	jsonPayload, _ := json.Marshal(payload)
	
	// Create request
	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("POST", url, strings.NewReader(string(jsonPayload)))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("authkey", s.msg91Config.AuthKey)
	
	// Make request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send MSG91 SMS: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode >= 400 {
		return fmt.Errorf("msg91 API error: %d", resp.StatusCode)
	}
	
	utils.Info("SMS sent via MSG91", zap.String("phone", phone[:6]+"****"))
	return nil
}

// getMSG91TemplateID returns template ID based on message type
// In production, you'd use proper DLT registered templates
func (s *SMSService) getMSG91TemplateID(message string) string {
	// Default template ID - configure in environment
	return utils.GetEnv("MSG91_TEMPLATE_ID", "1234567890")
}

// ShouldSendSMS determines if SMS should be sent based on user preference and context
func (s *SMSService) ShouldSendSMS(userPreference string, isCritical bool) bool {
	if isCritical {
		return true // Always send for critical alerts
	}
	
	if userPreference == "none" {
		return false // User opted out
	}
	
	return true
}

// SendBulkSMS sends SMS to multiple recipients (for admin alerts)
func (s *SMSService) SendBulkSMS(phones []string, message string) error {
	for _, phone := range phones {
		if err := s.SendSMS(phone, message); err != nil {
			utils.Error("Failed to send bulk SMS", zap.String("phone", phone), zap.Error(err))
			// Continue sending to others
		}
	}
	return nil
}

// GetDeliveryStatus would check delivery status (placeholder)
func (s *SMSService) GetDeliveryStatus(messageID string) (string, error) {
	// Implementation would depend on provider APIs
	return "delivered", nil
}

// SMSServiceInstance global instance
var SMSServiceInstance *SMSService

// InitSMSService initializes the global SMS service
func InitSMSService() {
	SMSServiceInstance = NewSMSService()
}
