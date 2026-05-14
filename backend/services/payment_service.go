package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"rapido-backend/config"
	"rapido-backend/database"
	"rapido-backend/models"
	"rapido-backend/utils"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PaymentService struct {
	DB *gorm.DB
}

func NewPaymentService() *PaymentService {
	return &PaymentService{DB: database.DB}
}

// ProcessPayment processes a payment for a ride
func (s *PaymentService) ProcessPayment(rideID uuid.UUID, method string, idempotencyKey string) (*models.Payment, error) {
	var result *models.Payment
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if idempotencyKey != "" {
			var existing models.Payment
			if err := tx.Where("idempotency_key = ?", idempotencyKey).First(&existing).Error; err == nil {
				result = &existing
				return nil
			}
		}

		var ride models.Ride
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&ride, rideID).Error; err != nil {
			return err
		}

		if ride.Status != models.RideStatusCompleted {
			return errors.New("ride must be completed before payment")
		}
		if ride.PaymentStatus == models.PaymentStatusCompleted {
			return errors.New("payment already completed")
		}

		payment := &models.Payment{
			RideID:         rideID,
			PayerID:        ride.RiderID,
			Amount:         ride.FinalFare,
			Currency:       "INR",
			Method:         method,
			Status:         models.PaymentStatusPending,
			IdempotencyKey: idempotencyKey,
		}
		if ride.DriverID != nil {
			payment.PayeeID = *ride.DriverID
		}

		switch method {
		case models.PaymentMethodCash:
			payment.Status = models.PaymentStatusCompleted
			payment.TransactionID = utils.GenerateIdempotencyKey()
		case models.PaymentMethodWallet:
			if err := s.processWalletPaymentTx(tx, ride.RiderID, ride.FinalFare, payment); err != nil {
				payment.Status = models.PaymentStatusFailed
				payment.FailureReason = err.Error()
			}
		case models.PaymentMethodUPI, models.PaymentMethodCard:
			payment.Status = models.PaymentStatusCompleted
			payment.TransactionID = utils.GenerateIdempotencyKey()
			payment.Gateway = "razorpay"
		default:
			return errors.New("invalid payment method")
		}

		if err := tx.Create(payment).Error; err != nil {
			return err
		}

		if payment.Status == models.PaymentStatusCompleted {
			ledger := NewLedgerService()
			sourceAccountType := models.LedgerAccountTypePaymentClearing
			var sourceAccountID *uuid.UUID
			if method == models.PaymentMethodWallet {
				sourceAccountType = models.LedgerAccountTypeUserWallet
				sourceAccountID = &ride.RiderID
			}

			driverPayout := ride.FinalFare - ride.PlatformFee
			if driverPayout < 0 {
				driverPayout = 0
			}

			if ride.DriverID != nil {
				if err := ledger.RecordRideSettlement(tx, sourceAccountType, sourceAccountID, ride.RiderID, *ride.DriverID, ride.FinalFare, driverPayout, ride.PlatformFee, rideID.String()); err != nil {
					return err
				}
			}

			if err := tx.Model(&ride).Update("payment_status", models.PaymentStatusCompleted).Error; err != nil {
				return err
			}

			transaction := &models.Transaction{
				UserID:      ride.RiderID,
				Type:        models.TransactionTypeRidePayment,
				Amount:      -ride.FinalFare,
				Currency:    "INR",
				Status:      models.PaymentStatusCompleted,
				Description: "Ride payment",
				ReferenceID: rideID.String(),
				PaymentID:   &payment.ID,
			}
			if err := tx.Create(transaction).Error; err != nil {
				return err
			}

			if ride.DriverID != nil {
				if err := s.creditDriverWalletTx(tx, *ride.DriverID, driverPayout, rideID); err != nil {
					return err
				}
			}

			if err := s.generateInvoiceTx(tx, &ride, payment); err != nil {
				return err
			}
		}

		result = payment
		return nil
	})

	if err != nil {
		return nil, err
	}
	return result, nil
}

// processWalletPayment processes wallet payment
func (s *PaymentService) processWalletPayment(userID uuid.UUID, amount float64, payment *models.Payment) error {
	var wallet models.Wallet
	if err := s.DB.Where("user_id = ?", userID).First(&wallet).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("wallet not found")
		}
		return err
	}

	if wallet.Balance < amount {
		return errors.New("insufficient wallet balance")
	}

	// Deduct from wallet
	wallet.Balance -= amount
	if err := s.DB.Save(&wallet).Error; err != nil {
		return err
	}

	// Update payment status
	payment.Status = models.PaymentStatusCompleted
	payment.TransactionID = utils.GenerateIdempotencyKey()

	// Record wallet transaction
	transaction := &models.Transaction{
		UserID:        userID,
		Type:          models.TransactionTypeWalletTopup,
		Amount:        -amount,
		Currency:      "INR",
		Status:        "completed",
		Description:   "Wallet payment for ride",
		WalletBalance: wallet.Balance,
	}
	s.DB.Create(transaction)

	return nil
}

func (s *PaymentService) processWalletPaymentTx(tx *gorm.DB, userID uuid.UUID, amount float64, payment *models.Payment) error {
	var wallet models.Wallet
	if err := tx.Where("user_id = ?", userID).First(&wallet).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("wallet not found")
		}
		return err
	}

	if wallet.Balance < amount {
		return errors.New("insufficient wallet balance")
	}

	wallet.Balance -= amount
	if err := tx.Save(&wallet).Error; err != nil {
		return err
	}

	payment.Status = models.PaymentStatusCompleted
	payment.TransactionID = utils.GenerateIdempotencyKey()

	transaction := &models.Transaction{
		UserID:        userID,
		Type:          models.TransactionTypeRidePayment,
		Amount:        -amount,
		Currency:      "INR",
		Status:        models.PaymentStatusCompleted,
		Description:   "Wallet payment for ride",
		WalletBalance: wallet.Balance,
	}
	return tx.Create(transaction).Error
}

// creditDriverWallet credits driver's wallet
func (s *PaymentService) creditDriverWallet(driverID uuid.UUID, amount float64, rideID uuid.UUID) error {
	var wallet models.Wallet
	if err := s.DB.Where("user_id = ?", driverID).First(&wallet).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create wallet
			wallet = models.Wallet{
				UserID:   driverID,
				Balance:  0,
				Currency: "INR",
			}
			s.DB.Create(&wallet)
		} else {
			return err
		}
	}

	wallet.Balance += amount
	if err := s.DB.Save(&wallet).Error; err != nil {
		return err
	}

	// Record transaction
	transaction := &models.Transaction{
		UserID:        driverID,
		Type:          models.TransactionTypeDriverPayout,
		Amount:        amount,
		Currency:      "INR",
		Status:        "completed",
		Description:   "Ride earnings",
		ReferenceID:   rideID.String(),
		WalletBalance: wallet.Balance,
	}
	s.DB.Create(transaction)

	return nil
}

func (s *PaymentService) creditDriverWalletTx(tx *gorm.DB, driverID uuid.UUID, amount float64, rideID uuid.UUID) error {
	var wallet models.Wallet
	if err := tx.Where("user_id = ?", driverID).First(&wallet).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			wallet = models.Wallet{UserID: driverID, Balance: 0, Currency: "INR"}
			if err := tx.Create(&wallet).Error; err != nil {
				return err
			}
		} else {
			return err
		}
	}

	wallet.Balance += amount
	if err := tx.Save(&wallet).Error; err != nil {
		return err
	}

	transaction := &models.Transaction{
		UserID:        driverID,
		Type:          models.TransactionTypeDriverPayout,
		Amount:        amount,
		Currency:      "INR",
		Status:        models.PaymentStatusCompleted,
		Description:   "Ride earnings",
		ReferenceID:   rideID.String(),
		WalletBalance: wallet.Balance,
	}
	return tx.Create(transaction).Error
}

// invoiceGSTPercent returns the GST rate from config, defaulting to 18%.
func invoiceGSTPercent() float64 {
	if v := config.Get().App.InvoiceGSTPercent; v > 0 {
		return v
	}
	return 18
}

// generateInvoice generates invoice for payment
func (s *PaymentService) generateInvoice(ride *models.Ride, payment *models.Payment) error {
	gst := invoiceGSTPercent()
	invoice := &models.Invoice{
		RideID:        ride.ID,
		PaymentID:     payment.ID,
		InvoiceNumber: "INV-" + ride.ID.String()[:8],
		CustomerName:  ride.Rider.Name,
		Amount:        ride.FinalFare,
		TaxAmount:     ride.FinalFare * gst / 100,
		TotalAmount:   ride.FinalFare * (1 + gst/100),
		GSTPercent:    gst,
		GeneratedAt:   time.Now(),
	}

	return s.DB.Create(invoice).Error
}

func (s *PaymentService) generateInvoiceTx(tx *gorm.DB, ride *models.Ride, payment *models.Payment) error {
	gst := invoiceGSTPercent()
	invoice := &models.Invoice{
		RideID:        ride.ID,
		PaymentID:     payment.ID,
		InvoiceNumber: "INV-" + ride.ID.String()[:8],
		CustomerName:  ride.Rider.Name,
		Amount:        ride.FinalFare,
		TaxAmount:     ride.FinalFare * gst / 100,
		TotalAmount:   ride.FinalFare * (1 + gst/100),
		GSTPercent:    gst,
		GeneratedAt:   time.Now(),
	}

	return tx.Create(invoice).Error
}

// GetWallet gets user's wallet
func (s *PaymentService) GetWallet(userID uuid.UUID) (*models.Wallet, error) {
	var wallet models.Wallet
	if err := s.DB.Where("user_id = ?", userID).First(&wallet).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create wallet
			wallet = models.Wallet{
				UserID:   userID,
				Balance:  0,
				Currency: "INR",
			}
			if err := s.DB.Create(&wallet).Error; err != nil {
				return nil, err
			}
			return &wallet, nil
		}
		return nil, err
	}
	return &wallet, nil
}

// AddMoneyToWallet adds money to wallet
func (s *PaymentService) AddMoneyToWallet(userID uuid.UUID, amount float64, method string) (*models.Transaction, error) {
	var wallet models.Wallet
	if err := s.DB.Where("user_id = ?", userID).First(&wallet).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			wallet = models.Wallet{
				UserID:   userID,
				Balance:  0,
				Currency: "INR",
			}
			s.DB.Create(&wallet)
		} else {
			return nil, err
		}
	}

	// In production, integrate with payment gateway
	// For now, just add to wallet
	wallet.Balance += amount
	if err := s.DB.Save(&wallet).Error; err != nil {
		return nil, err
	}

	transaction := &models.Transaction{
		UserID:        userID,
		Type:          models.TransactionTypeWalletTopup,
		Amount:        amount,
		Currency:      "INR",
		Status:        "completed",
		Description:   "Wallet topup via " + method,
		WalletBalance: wallet.Balance,
	}

	if err := s.DB.Create(transaction).Error; err != nil {
		return nil, err
	}

	if err := NewLedgerService().RecordTopup(s.DB, userID, amount, transaction.ID.String()); err != nil {
		return nil, err
	}

	return transaction, nil
}

// GetTransactionHistory gets transaction history
func (s *PaymentService) GetTransactionHistory(userID uuid.UUID, page, perPage int) ([]models.Transaction, int64, error) {
	var transactions []models.Transaction
	var count int64

	offset := (page - 1) * perPage

	s.DB.Model(&models.Transaction{}).Where("user_id = ?", userID).Count(&count)

	if err := s.DB.Where("user_id = ?", userID).
		Order("created_at DESC").
		Offset(offset).Limit(perPage).
		Find(&transactions).Error; err != nil {
		return nil, 0, err
	}

	return transactions, count, nil
}

// RequestWithdrawal requests driver withdrawal
func (s *PaymentService) RequestWithdrawal(driverID uuid.UUID, amount float64, method string, bankDetails map[string]interface{}) (*models.Withdrawal, error) {
	var earnings models.DriverEarnings
	if err := s.DB.Where("driver_id = ?", driverID).First(&earnings).Error; err != nil {
		return nil, err
	}

	if earnings.CurrentBalance < amount {
		return nil, errors.New("insufficient balance")
	}

	withdrawal := &models.Withdrawal{
		DriverID:    driverID,
		Amount:      amount,
		Currency:    "INR",
		Method:      method,
		BankDetails: bankDetails,
		Status:      "pending",
		RequestedAt: time.Now(),
	}

	if err := s.DB.Create(withdrawal).Error; err != nil {
		return nil, err
	}

	// Update pending amount
	s.DB.Model(&earnings).Update("pending_amount", gorm.Expr("pending_amount + ?", amount))

	return withdrawal, nil
}

// ProcessWithdrawal (Admin) processes withdrawal
func (s *PaymentService) ProcessWithdrawal(withdrawalID, adminID uuid.UUID, approved bool, rejectionReason string) error {
	var withdrawal models.Withdrawal
	if err := s.DB.First(&withdrawal, withdrawalID).Error; err != nil {
		return err
	}

	if withdrawal.Status != "pending" {
		return errors.New("withdrawal already processed")
	}

	now := time.Now()

	if approved {
		withdrawal.Status = "completed"
		withdrawal.ProcessedAt = &now
		withdrawal.ProcessedBy = &adminID

		// Update driver earnings
		s.DB.Model(&models.DriverEarnings{}).Where("driver_id = ?", withdrawal.DriverID).
			Updates(map[string]interface{}{
				"current_balance":  gorm.Expr("current_balance - ?", withdrawal.Amount),
				"pending_amount":   gorm.Expr("pending_amount - ?", withdrawal.Amount),
				"withdrawn_amount": gorm.Expr("withdrawn_amount + ?", withdrawal.Amount),
			})

		// Record transaction
		transaction := &models.Transaction{
			UserID:      withdrawal.DriverID,
			Type:        models.TransactionTypeWalletWithdrawal,
			Amount:      -withdrawal.Amount,
			Currency:    "INR",
			Status:      "completed",
			Description: "Withdrawal to " + withdrawal.Method,
		}
		s.DB.Create(transaction)
		_ = NewLedgerService().RecordWithdrawal(s.DB, withdrawal.DriverID, withdrawal.Amount, withdrawalID.String())
	} else {
		withdrawal.Status = "rejected"
		withdrawal.RejectionReason = rejectionReason
		withdrawal.ProcessedAt = &now
		withdrawal.ProcessedBy = &adminID

		// Return pending amount to available
		s.DB.Model(&models.DriverEarnings{}).Where("driver_id = ?", withdrawal.DriverID).
			Update("pending_amount", gorm.Expr("pending_amount - ?", withdrawal.Amount))
	}

	return s.DB.Save(&withdrawal).Error
}

// GetPaymentByRide gets payment for a ride
func (s *PaymentService) GetPaymentByRide(rideID uuid.UUID) (*models.Payment, error) {
	var payment models.Payment
	if err := s.DB.Where("ride_id = ?", rideID).First(&payment).Error; err != nil {
		return nil, err
	}
	return &payment, nil
}

// RetryFailedPayment retries the latest failed payment for a ride.
func (s *PaymentService) RetryFailedPayment(rideID uuid.UUID, method string, idempotencyKey string) (*models.Payment, error) {
	var failedPayment models.Payment
	if err := s.DB.Where("ride_id = ? AND status = ?", rideID, models.PaymentStatusFailed).
		Order("created_at DESC").
		First(&failedPayment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("no failed payment found for this ride")
		}
		return nil, err
	}

	if method == "" {
		method = failedPayment.Method
	}
	if idempotencyKey == "" {
		idempotencyKey = utils.GenerateIdempotencyKey()
	}

	return s.ProcessPayment(rideID, method, idempotencyKey)
}

// RefundPayment credits rider wallet and records corresponding ledger entries.
func (s *PaymentService) RefundPayment(paymentID, requesterID uuid.UUID, amount float64, reason string) (*models.Payment, error) {
	var updatedPayment models.Payment

	err := s.DB.Transaction(func(tx *gorm.DB) error {
		var payment models.Payment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&payment, paymentID).Error; err != nil {
			return err
		}

		if payment.Status != models.PaymentStatusCompleted && payment.Status != models.PaymentStatusRefunded {
			return errors.New("only completed payments can be refunded")
		}

		if requesterID != payment.PayerID {
			var requester models.User
			if err := tx.Select("role").First(&requester, requesterID).Error; err != nil {
				return errors.New("requester not authorized")
			}
			if requester.Role != "admin" {
				return errors.New("requester not authorized")
			}
		}

		remaining := payment.Amount - payment.RefundAmount
		if remaining <= 0 {
			return errors.New("payment is already fully refunded")
		}
		if amount > remaining {
			return errors.New("refund amount exceeds refundable balance")
		}

		var wallet models.Wallet
		if err := tx.Where("user_id = ?", payment.PayerID).First(&wallet).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				wallet = models.Wallet{UserID: payment.PayerID, Balance: 0, Currency: "INR"}
				if err := tx.Create(&wallet).Error; err != nil {
					return err
				}
			} else {
				return err
			}
		}

		wallet.Balance += amount
		if err := tx.Save(&wallet).Error; err != nil {
			return err
		}

		transaction := &models.Transaction{
			UserID:        payment.PayerID,
			Type:          models.TransactionTypeRefund,
			Amount:        amount,
			Currency:      "INR",
			Status:        models.PaymentStatusCompleted,
			Description:   "Payment refund",
			ReferenceID:   payment.ID.String(),
			PaymentID:     &payment.ID,
			WalletBalance: wallet.Balance,
			Metadata: models.JSONMap{
				"reason": reason,
			},
		}
		if err := tx.Create(transaction).Error; err != nil {
			return err
		}

		if err := NewLedgerService().RecordRefund(tx, payment.PayerID, amount, payment.ID.String()); err != nil {
			return err
		}

		payment.RefundAmount += amount
		now := time.Now()
		payment.RefundedAt = &now
		if payment.Metadata == nil {
			payment.Metadata = models.JSONMap{}
		}
		if reason != "" {
			payment.Metadata["refund_reason"] = reason
		}

		if payment.RefundAmount >= payment.Amount {
			payment.Status = models.PaymentStatusRefunded
		}

		if err := tx.Save(&payment).Error; err != nil {
			return err
		}

		updatedPayment = payment
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &updatedPayment, nil
}

// ProcessWebhook processes payment webhooks from Razorpay/Stripe
func (s *PaymentService) ProcessWebhook(signature string, body []byte) (map[string]interface{}, error) {
	// In production, verify webhook signature
	// For now, parse and process the event

	var event map[string]interface{}
	if err := json.Unmarshal(body, &event); err != nil {
		return nil, fmt.Errorf("invalid webhook payload: %w", err)
	}

	eventType, _ := event["event"].(string)
	if eventType == "" {
		eventType, _ = event["type"].(string) // Stripe format
	}

	// Process based on event type
	switch eventType {
	case "payment.captured", "payment_intent.succeeded":
		// Update payment status
		payload, _ := event["payload"].(map[string]interface{})
		if payment, ok := payload["payment"].(map[string]interface{}); ok {
			paymentID, _ := payment["id"].(string)
			// Update payment in DB
			s.DB.Model(&models.Payment{}).Where("gateway_payment_id = ?", paymentID).
				Updates(map[string]interface{}{"status": "completed"})
		}
	case "payment.failed", "payment_intent.payment_failed":
		// Handle failed payment
		payload, _ := event["payload"].(map[string]interface{})
		if payment, ok := payload["payment"].(map[string]interface{}); ok {
			paymentID, _ := payment["id"].(string)
			s.DB.Model(&models.Payment{}).Where("gateway_payment_id = ?", paymentID).
				Updates(map[string]interface{}{"status": "failed"})
		}
	}

	return map[string]interface{}{
		"event_type": eventType,
		"processed":  true,
		"timestamp":  time.Now(),
	}, nil
}
