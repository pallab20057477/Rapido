# Rapido API - Postman Test Collection

**Base URL:** `http://localhost:8080` (or your deployed URL)  
**Environment Variables:**
- `{{base_url}}` - http://localhost:8080
- `{{access_token}}` - JWT token from login
- `{{driver_token}}` - Driver JWT token
- `{{admin_token}}` - Admin JWT token

---

## � API Usage Guidelines

### 🔑 Idempotency-Key Header (Required for Critical Operations)

For the following operations, **always** include an `Idempotency-Key` header:
- `POST /rides` (ride creation)
- `POST /rides/:id/pay` (payments)
- `POST /wallet/add-money` (wallet recharge)
- `POST /payments/:id/refund` (refunds)
- `POST /rides/:id/retry` (retry matching)

```http
POST {{base_url}}/api/v1/rides
Content-Type: application/json
Idempotency-Key: {{$guid}}
Authorization: Bearer {{access_token}}

{ ... }
```

### 📄 Pagination Query Parameters

List endpoints support pagination:
```
GET /rides/history?page=1&limit=20
GET /notifications?page=1&limit=20
GET /transactions?page=1&limit=20
GET /admin/rides?page=1&limit=50&status=completed
```

**Parameters:**
- `page` - Page number (default: 1)
- `limit` - Items per page (default: 20, max: 100)
- `sort` - Sort order (`?sort=-created_at` for descending)

### 🔍 Filtering Examples

```
GET /admin/rides?status=completed&from=2026-01-01&to=2026-01-31
GET /rides/history?status=cancelled
GET /drivers/:id/reviews?rating=5&page=1&limit=10
```

### ⏱️ Rate Limit Headers

Responses include rate limit information:
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1649251200
```

---

## � 1. Authentication APIs

### 1.1 Request OTP
```http
POST {{base_url}}/api/v1/auth/otp/request
Content-Type: application/json

{
  "phone": "+919876543210",
  "country_code": "+91"
}

# Response (OTP returned for testing - remove in production):
{
  "success": true,
  "message": "OTP sent successfully",
  "data": {
    "phone": "+91******3210",
    "expires_in": 300,
    "otp": "847291"
  }
}
```

**Note:** Copy the `otp` value from this response to use in Step 1.2

### 1.2 Verify OTP
```http
POST {{base_url}}/api/v1/auth/otp/verify
Content-Type: application/json

{
  "phone": "+919876543210",
  "email": "user@example.com",
  "otp": "{{otp_from_step_1}}"
}

# Response:
{
  "success": true,
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
  "expires_in": 86400,
  "user": {
    "id": "uuid",
    "phone": "+919876543210",
    "name": "Rahul Kumar"
  }
}
```

### 1.3 Google Login
```http
POST {{base_url}}/api/v1/auth/google
Content-Type: application/json

{
  "id_token": "google_id_token_here",
  "device_id": "device_123",
  "device_type": "android",
  "fcm_token": "firebase_token_here"
}
```

### 1.4 Refresh Token
```http
POST {{base_url}}/api/v1/auth/refresh
Content-Type: application/json

{
  "refresh_token": "{{refresh_token}}"
}
```

### 1.5 Logout
```http
POST {{base_url}}/api/v1/auth/logout
Authorization: Bearer {{access_token}}
Content-Type: application/json

{
  "device_id": "device_123"
}
```

### 1.6 Get Profile
```http
GET {{base_url}}/api/v1/auth/profile
Authorization: Bearer {{access_token}}
```

### 1.7 Update Profile
```http
PATCH {{base_url}}/api/v1/auth/profile
Authorization: Bearer {{access_token}}
Content-Type: application/json

{
  "name": "Rahul Kumar",
  "email": "rahul@example.com",
  "profile_image": "https://example.com/image.jpg"
}
```

---

## 📞 2. Emergency Contacts APIs

### 2.1 Add Emergency Contact
```http
POST {{base_url}}/api/v1/auth/emergency-contacts
Authorization: Bearer {{access_token}}
Content-Type: application/json

{
  "name": "Mom",
  "phone": "+919876543211",
  "relationship": "mother",
  "priority": 1
}
```

### 2.2 Get All Emergency Contacts
```http
GET {{base_url}}/api/v1/auth/emergency-contacts
Authorization: Bearer {{access_token}}
```

### 2.3 Update Emergency Contact
```http
PUT {{base_url}}/api/v1/auth/emergency-contacts/{{contact_id}}
Authorization: Bearer {{access_token}}
Content-Type: application/json

{
  "name": "Mom Updated",
  "phone": "+919876543211",
  "relationship": "mother",
  "priority": 1
}
```

### 2.4 Delete Emergency Contact
```http
DELETE {{base_url}}/api/v1/auth/emergency-contacts/{{contact_id}}
Authorization: Bearer {{access_token}}
```

---

## 🆘 3. SOS APIs

### 3.1 Trigger SOS
```http
POST {{base_url}}/api/v1/sos/trigger
Authorization: Bearer {{access_token}}
Content-Type: application/json

{
  "ride_id": "ride_uuid_here",
  "reason": "emergency",
  "location": {
    "lat": 19.0760,
    "lng": 72.8777,
    "address": "Mumbai, Andheri East"
  },
  "message": "Need immediate help"
}
```

### 3.2 Get SOS History
```http
GET {{base_url}}/api/v1/sos/history
Authorization: Bearer {{access_token}}
```

---

## ⭐ 4. Rating APIs

### 4.1 Submit Rating
```http
POST {{base_url}}/api/v1/rides/{{ride_id}}/rate
Authorization: Bearer {{access_token}}
Content-Type: application/json

{
  "rating": 5,
  "review": "Great ride! Driver was very professional.",
  "categories": {
    "driving": 5,
    "cleanliness": 5,
    "behavior": 5
  },
  "anonymous": false
}
```

### 4.2 Get My Rating for a Ride
```http
GET {{base_url}}/api/v1/rides/{{ride_id}}/my-rating
Authorization: Bearer {{access_token}}
```

### 4.3 Get Driver Reviews (Public)
```http
GET {{base_url}}/api/v1/drivers/{{driver_id}}/reviews?page=1&per_page=10
Authorization: Bearer {{access_token}}
```

### 4.4 Get Driver Rating Summary
```http
GET {{base_url}}/api/v1/drivers/{{driver_id}}/rating-summary
Authorization: Bearer {{access_token}}
```

### 4.5 Report a Rating
```http
POST {{base_url}}/api/v1/ratings/{{rating_id}}/report
Authorization: Bearer {{access_token}}
Content-Type: application/json

{
  "reason": "inappropriate_content",
  "description": "This review contains abusive language"
}
```

---

## 🎫 5. Support Ticket APIs

### 5.1 Create Support Ticket
```http
POST {{base_url}}/api/v1/users/support/tickets
Authorization: Bearer {{access_token}}
Content-Type: application/json

{
  "category": "payment_issue",
  "priority": "high",
  "subject": "Payment not reflected",
  "description": "I paid but the ride shows pending payment",
  "ride_id": "ride_uuid_here"
}
```

### 5.2 Get My Tickets
```http
GET {{base_url}}/api/v1/users/support/tickets?status=open&page=1
Authorization: Bearer {{access_token}}
```

### 5.3 Get Ticket Details
```http
GET {{base_url}}/api/v1/users/support/tickets/{{ticket_id}}
Authorization: Bearer {{access_token}}
```

### 5.4 Add Message to Ticket
```http
POST {{base_url}}/api/v1/users/support/tickets/{{ticket_id}}/messages
Authorization: Bearer {{access_token}}
Content-Type: application/json

{
  "message": "I have attached the payment screenshot",
  "attachments": ["url_to_image.jpg"]
}
```

---

## 💳 6. Payment Method APIs

### 6.1 Add Card
```http
POST {{base_url}}/api/v1/payments/methods/card
Authorization: Bearer {{access_token}}
Content-Type: application/json

{
  "card_number": "4111111111111111",
  "expiry_month": "12",
  "expiry_year": "2026",
  "cvv": "123",
  "card_holder_name": "Rahul Kumar",
  "is_default": true
}
```

### 6.2 Add UPI
```http
POST {{base_url}}/api/v1/payments/methods/upi
Authorization: Bearer {{access_token}}
Content-Type: application/json

{
  "upi_id": "rahul@upi",
  "is_default": false
}
```

### 6.3 Get All Payment Methods
```http
GET {{base_url}}/api/v1/payments/methods
Authorization: Bearer {{access_token}}
```

### 6.4 Remove Payment Method
```http
DELETE {{base_url}}/api/v1/payments/methods/{{payment_method_id}}
Authorization: Bearer {{access_token}}
```

### 6.5 Set Default Payment Method
```http
POST {{base_url}}/api/v1/payments/methods/{{payment_method_id}}/default
Authorization: Bearer {{access_token}}
```

---

## 📅 7. Scheduled Ride APIs

### 7.1 Schedule a Ride
```http
POST {{base_url}}/api/v1/rides/schedule
Authorization: Bearer {{access_token}}
Content-Type: application/json
Idempotency-Key: unique_key_here

{
  "vehicle_type": "bike",
  "pickup_lat": 19.0760,
  "pickup_lng": 72.8777,
  "pickup_address": "Andheri East, Mumbai",
  "dropoff_lat": 19.0178,
  "dropoff_lng": 72.8478,
  "dropoff_address": "Bandra West, Mumbai",
  "scheduled_at": "2026-05-10T08:00:00Z",
  "preferences": {
    "female_driver": false,
    "ac": false,
    "luggage": false
  }
}
```

### 7.2 Get Scheduled Rides
```http
GET {{base_url}}/api/v1/rides/scheduled?status=upcoming
Authorization: Bearer {{access_token}}
```

### 7.3 Get Scheduled Ride Details
```http
GET {{base_url}}/api/v1/rides/scheduled/{{scheduled_ride_id}}
Authorization: Bearer {{access_token}}
```

### 7.4 Update Scheduled Ride
```http
PUT {{base_url}}/api/v1/rides/scheduled/{{scheduled_ride_id}}
Authorization: Bearer {{access_token}}
Content-Type: application/json

{
  "scheduled_at": "2026-05-10T09:00:00Z",
  "preferences": {
    "female_driver": true,
    "ac": true,
    "luggage": true
  }
}
```

### 7.5 Cancel Scheduled Ride
```http
POST {{base_url}}/api/v1/rides/scheduled/{{scheduled_ride_id}}/cancel
Authorization: Bearer {{access_token}}
Content-Type: application/json

{
  "reason": "change_of_plans"
}
```

---

## 🚗 8. Ride APIs (Rider)

### 8.1 Request Ride
```http
POST {{base_url}}/api/v1/rides
Authorization: Bearer {{access_token}}
Content-Type: application/json
Idempotency-Key: ride_request_001

{
  "vehicle_type": "bike",
  "pickup_lat": 19.0760,
  "pickup_lng": 72.8777,
  "pickup_address": "Andheri East, Mumbai",
  "dropoff_lat": 19.0178,
  "dropoff_lng": 72.8478,
  "dropoff_address": "Bandra West, Mumbai",
  "payment_method": "wallet",
  "promo_code": "RAPIDO50",
  "preferences": {
    "female_driver": false,
    "ac": false,
    "luggage": false
  }
}
```

### 8.2 Get Active Ride
```http
GET {{base_url}}/api/v1/rides/active
Authorization: Bearer {{access_token}}
```

### 8.3 Get Ride History
```http
GET {{base_url}}/api/v1/rides/history?page=1&limit=10
Authorization: Bearer {{access_token}}
```

### 8.4 Get Ride Details
```http
GET {{base_url}}/api/v1/rides/{{ride_id}}
Authorization: Bearer {{access_token}}
```

### 8.5 Cancel Ride
```http
POST {{base_url}}/api/v1/rides/{{ride_id}}/cancel
Authorization: Bearer {{access_token}}
Content-Type: application/json
Idempotency-Key: cancel_001

{
  "reason": "driver_too_far",
  "notes": "Driver is taking too long"
}
```

### 8.6 Estimate Fare
```http
GET {{base_url}}/api/v1/rides/estimate?pickup_lat=19.0760&pickup_lng=72.8777&dropoff_lat=19.0178&dropoff_lng=72.8478&vehicle_type=bike
Authorization: Bearer {{access_token}}
```

### 8.7 Get Nearby Drivers
```http
GET {{base_url}}/api/v1/drivers/nearby?lat=19.0760&lng=72.8777&radius=3
Authorization: Bearer {{access_token}}
```

### 8.8 Track Ride (Real-time Location)
```http
GET {{base_url}}/api/v1/rides/{{ride_id}}/track
Authorization: Bearer {{access_token}}

# Response:
{
  "success": true,
  "data": {
    "ride_id": "ride_uuid",
    "status": "ongoing",
    "driver_location": {
      "lat": 19.0760,
      "lng": 72.8777
    },
    "pickup_lat": 19.0760,
    "pickup_lng": 72.8777,
    "dropoff_lat": 19.0178,
    "dropoff_lng": 72.8478,
    "updated_at": "2026-05-06T08:30:00Z"
  }
}
```

### 8.9 Get Ride ETA
```http
GET {{base_url}}/api/v1/rides/{{ride_id}}/eta
Authorization: Bearer {{access_token}}

# Response:
{
  "success": true,
  "data": {
    "ride_id": "ride_uuid",
    "status": "driver_assigned",
    "eta_minutes": 8,
    "eta_text": "8 mins",
    "updated_at": "2026-05-06T08:30:00Z"
  }
}
```

### 8.10 Retry Ride Matching
```http
POST {{base_url}}/api/v1/rides/{{ride_id}}/retry
Authorization: Bearer {{access_token}}
Content-Type: application/json
Idempotency-Key: retry_001

# Response:
{
  "success": true,
  "data": {
    "ride_id": "ride_uuid",
    "status": "retrying_match",
    "message": "Looking for drivers again...",
    "retry_count": 1
  }
}
```

### 8.11 Update Ride Status (RESTful PATCH)
```http
PATCH {{base_url}}/api/v1/rides/{{ride_id}}/status
Authorization: Bearer {{access_token}}
Content-Type: application/json

{
  "status": "started"
}

# Available statuses: accepted, started, completed, cancelled
```

### 8.12 Apply Promo Code
```http
POST {{base_url}}/api/v1/rides/{{ride_id}}/apply-promo
Authorization: Bearer {{access_token}}
Content-Type: application/json

{
  "promo_code": "RAPIDO50"
}

# Response:
{
  "success": true,
  "message": "Promo code applied",
  "data": {
    "ride_id": "ride_uuid",
    "promo_code": "RAPIDO50",
    "original_fare": 250.00,
    "discount_amount": 125.00,
    "final_fare": 125.00
  }
}
```

### 8.13 Get Fare Breakdown
```http
GET {{base_url}}/api/v1/rides/{{ride_id}}/fare
Authorization: Bearer {{access_token}}

# Response:
{
  "success": true,
  "data": {
    "ride_id": "ride_uuid",
    "base_fare": 30.00,
    "distance_charge": 45.00,
    "time_charge": 15.00,
    "surge_multiplier": 1.5,
    "surge_amount": 22.50,
    "platform_fee": 5.00,
    "tax_amount": 12.50,
    "discount_amount": 125.00,
    "promo_code": "RAPIDO50",
    "estimated_fare": 250.00,
    "final_fare": 125.00,
    "currency": "INR",
    "breakdown": {
      "base_fare": "₹30.00",
      "distance_charge": "₹45.00 (9.0 km × ₹5.00/km)",
      "time_charge": "₹15.00 (15 min × ₹1.00/min)",
      "surge": "₹22.50 (1.5x)",
      "platform_fee": "₹5.00",
      "tax": "₹12.50",
      "discount": "-₹125.00",
      "total": "₹125.00"
    }
  }
}
```

---

## 💰 9. Payment APIs (Rider)

### 9.1 Get Wallet Balance
```http
GET {{base_url}}/api/v1/wallet
Authorization: Bearer {{access_token}}
```

### 9.2 Add Money to Wallet
```http
POST {{base_url}}/api/v1/wallet/add-money
Authorization: Bearer {{access_token}}
Content-Type: application/json
Idempotency-Key: wallet_add_001

{
  "amount": 500,
  "payment_method": "card",
  "payment_method_id": "card_uuid_here"
}
```

### 9.3 Get Transaction History
```http
GET {{base_url}}/api/v1/transactions?page=1&limit=20
Authorization: Bearer {{access_token}}
```

### 9.4 Pay for Ride (Domain-Separated)
```http
POST {{base_url}}/api/v1/payments/rides/{{ride_id}}/pay
Authorization: Bearer {{access_token}}
Content-Type: application/json
Idempotency-Key: payment_001

{
  "method": "wallet",
  "payment_method_id": ""
}
```

### 9.5 Retry Failed Payment
```http
POST {{base_url}}/api/v1/payments/rides/{{ride_id}}/retry
Authorization: Bearer {{access_token}}
Content-Type: application/json
Idempotency-Key: payment_retry_001

{
  "method": "card",
  "payment_method_id": "card_uuid_here"
}
```

### 9.6 Get Payment Status
```http
GET {{base_url}}/api/v1/payments/rides/{{ride_id}}
Authorization: Bearer {{access_token}}
```

### 9.7 Request Refund
```http
POST {{base_url}}/api/v1/payments/{{payment_id}}/refund
Authorization: Bearer {{access_token}}
Content-Type: application/json
Idempotency-Key: refund_001

{
  "reason": "service_issue",
  "amount": 125.00
}
```

### 9.8 Request Withdrawal (Driver)
```http
POST {{base_url}}/api/v1/withdrawals
Authorization: Bearer {{driver_token}}
Content-Type: application/json
Idempotency-Key: withdrawal_001

{
  "amount": 1000,
  "bank_account_id": "bank_uuid_here"
}
```

### 9.9 Payment Webhook (Razorpay/Stripe)
```http
POST {{base_url}}/api/v1/payments/webhook
Content-Type: application/json
X-Razorpay-Signature: webhook_signature_here

{
  "event": "payment.captured",
  "payload": {
    "payment": {
      "id": "pay_1234567890",
      "status": "captured",
      "amount": 12500
    }
  }
}

# Response:
{
  "success": true,
  "event_type": "payment.captured",
  "processed": true
}
```

---

## 🏍️ 10. Driver APIs

### 10.1 Register as Driver
```http
POST {{base_url}}/api/v1/drivers/register
Authorization: Bearer {{access_token}}
Content-Type: multipart/form-data

{
  "license_number": "MH0120231234567",
  "license_expiry": "2028-05-01",
  "rc_number": "MH01AB1234",
  "vehicle_type": "bike",
  "vehicle_model": "Honda Activa",
  "vehicle_year": 2022,
  "documents": {
    "license_image": "file",
    "rc_image": "file",
    "aadhaar_image": "file"
  }
}
```

### 10.2 Get Driver Profile
```http
GET {{base_url}}/api/v1/drivers/profile
Authorization: Bearer {{driver_token}}
```

### 10.3 Update Driver Profile
```http
PATCH {{base_url}}/api/v1/drivers/profile
Authorization: Bearer {{driver_token}}
Content-Type: application/json

{
  "vehicle_model": "Honda Activa 6G",
  "preferred_locations": ["Andheri", "Bandra"],
  "languages": ["hindi", "english", "marathi"]
}
```

### 10.4 Go Online
```http
POST {{base_url}}/api/v1/drivers/online
Authorization: Bearer {{driver_token}}
Content-Type: application/json

{
  "lat": 19.0760,
  "lng": 72.8777,
  "vehicle_type": "bike"
}
```

### 10.5 Go Offline
```http
POST {{base_url}}/api/v1/drivers/offline
Authorization: Bearer {{driver_token}}
```

### 10.6 Update Location (Driver)
```http
POST {{base_url}}/api/v1/drivers/location
Authorization: Bearer {{driver_token}}
Content-Type: application/json

{
  "lat": 19.0760,
  "lng": 72.8777,
  "heading": 90,
  "speed": 25,
  "accuracy": 10,
  "battery_level": 80
}
```

### 10.7 Get Earnings
```http
GET {{base_url}}/api/v1/drivers/earnings?period=today
Authorization: Bearer {{driver_token}}
```

### 10.8 Get Driver Stats
```http
GET {{base_url}}/api/v1/drivers/stats
Authorization: Bearer {{driver_token}}
```

---

## 🏍️ 11. Driver Ride Actions

### 11.1 Accept Ride
```http
POST {{base_url}}/api/v1/rides/{{ride_id}}/accept
Authorization: Bearer {{driver_token}}
Content-Type: application/json
Idempotency-Key: accept_001

{
  "lat": 19.0760,
  "lng": 72.8777
}
```

### 11.2 Reject Ride
```http
POST {{base_url}}/api/v1/rides/{{ride_id}}/reject
Authorization: Bearer {{driver_token}}
Content-Type: application/json
Idempotency-Key: reject_001

{
  "reason": "too_far"
}
```

### 11.3 Mark Arrived
```http
POST {{base_url}}/api/v1/rides/{{ride_id}}/arrived
Authorization: Bearer {{driver_token}}
Content-Type: application/json
Idempotency-Key: arrived_001

{
  "lat": 19.0760,
  "lng": 72.8777
}
```

### 11.4 Start Ride
```http
POST {{base_url}}/api/v1/rides/{{ride_id}}/start
Authorization: Bearer {{driver_token}}
Content-Type: application/json
Idempotency-Key: start_001

{
  "otp": "1234",
  "lat": 19.0760,
  "lng": 72.8777
}
```

### 11.5 Complete Ride
```http
POST {{base_url}}/api/v1/rides/{{ride_id}}/complete
Authorization: Bearer {{driver_token}}
Content-Type: application/json
Idempotency-Key: complete_001

{
  "lat": 19.0178,
  "lng": 72.8478,
  "final_fare": 125.50
}
```

### 11.6 Update Ride Location (Driver)
```http
POST {{base_url}}/api/v1/rides/{{ride_id}}/location
Authorization: Bearer {{driver_token}}
Content-Type: application/json

{
  "lat": 19.0500,
  "lng": 72.8600,
  "heading": 180
}
```

---

## 👨‍💼 12. Admin APIs

### 12.1 Get Dashboard Stats
```http
GET {{base_url}}/api/v1/admin/dashboard
Authorization: Bearer {{admin_token}}
```

### 12.2 Get All Rides
```http
GET {{base_url}}/api/v1/admin/rides?status=active&page=1&limit=20
Authorization: Bearer {{admin_token}}
```

### 12.3 Get All Users
```http
GET {{base_url}}/api/v1/admin/users?page=1&limit=50
Authorization: Bearer {{admin_token}}
```

### 12.4 Get All Drivers
```http
GET {{base_url}}/api/v1/admin/drivers?status=pending&page=1
Authorization: Bearer {{admin_token}}
```

### 12.5 Get Pending Verifications
```http
GET {{base_url}}/api/v1/admin/drivers/pending
Authorization: Bearer {{admin_token}}
```

### 12.6 Verify Driver
```http
POST {{base_url}}/api/v1/admin/drivers/verify
Authorization: Bearer {{admin_token}}
Content-Type: application/json

{
  "driver_id": "driver_uuid_here",
  "status": "approved",
  "notes": "Documents verified successfully"
}
```

### 12.7 Get All Payments
```http
GET {{base_url}}/api/v1/admin/payments?status=success&date_from=2026-05-01&date_to=2026-05-31
Authorization: Bearer {{admin_token}}
```

### 12.8 Get Pending Withdrawals
```http
GET {{base_url}}/api/v1/admin/withdrawals/pending
Authorization: Bearer {{admin_token}}
```

### 12.9 Process Withdrawal
```http
POST {{base_url}}/api/v1/admin/withdrawals/process
Authorization: Bearer {{admin_token}}
Content-Type: application/json

{
  "withdrawal_id": "withdrawal_uuid_here",
  "action": "approve",
  "notes": "Processed via NEFT"
}
```

### 12.10 Create Surge Pricing
```http
POST {{base_url}}/api/v1/admin/surge-pricing
Authorization: Bearer {{admin_token}}
Content-Type: application/json

{
  "area_name": "Andheri",
  "lat": 19.0760,
  "lng": 72.8777,
  "radius_km": 5,
  "multiplier": 1.5,
  "reason": "high_demand",
  "start_time": "2026-05-06T08:00:00Z",
  "end_time": "2026-05-06T10:00:00Z"
}
```

### 12.11 Remove Surge Pricing
```http
DELETE {{base_url}}/api/v1/admin/surge-pricing/{{surge_id}}
Authorization: Bearer {{admin_token}}
```

### 12.12 Create Promo Code
```http
POST {{base_url}}/api/v1/admin/promo-codes
Authorization: Bearer {{admin_token}}
Content-Type: application/json

{
  "code": "RAPIDO50",
  "description": "50% off up to ₹50",
  "discount_type": "percentage",
  "discount_value": 50,
  "max_discount": 50,
  "min_order_value": 100,
  "valid_from": "2026-05-01",
  "valid_until": "2026-05-31",
  "usage_limit": 10000,
  "per_user_limit": 3
}
```

### 12.13 Get Reports
```http
GET {{base_url}}/api/v1/admin/reports?type=rides&date_from=2026-05-01&date_to=2026-05-31
Authorization: Bearer {{admin_token}}
```

---

## 📊 13. Ledger APIs (Admin)

### 13.1 Get Ledger Accounts
```http
GET {{base_url}}/api/v1/admin/ledger/accounts
Authorization: Bearer {{admin_token}}
```

### 13.2 Get Ledger Entries
```http
GET {{base_url}}/api/v1/admin/ledger/entries?account_id={{account_id}}&date_from=2026-05-01
Authorization: Bearer {{admin_token}}
```

### 13.3 Audit Ledger Batch
```http
POST {{base_url}}/api/v1/admin/ledger/audit-batch
Authorization: Bearer {{admin_token}}
Content-Type: application/json

{
  "start_date": "2026-05-01",
  "end_date": "2026-05-31"
}
```

### 13.4 Get Account Balance
```http
GET {{base_url}}/api/v1/admin/ledger/account-balance?account_id={{account_id}}
Authorization: Bearer {{admin_token}}
```

---

## 🆘 14. Admin SOS APIs

### 14.1 Get Active SOS Events
```http
GET {{base_url}}/api/v1/admin/sos/active?page=1&limit=20
Authorization: Bearer {{admin_token}}
```

### 14.2 Resolve SOS
```http
POST {{base_url}}/api/v1/admin/sos/{{sos_id}}/resolve
Authorization: Bearer {{admin_token}}
Content-Type: application/json

{
  "resolution": "driver_contacted",
  "notes": "Driver confirmed rider is safe"
}
```

---

## 🎫 15. Admin Support APIs

### 15.1 Get All Tickets
```http
GET {{base_url}}/api/v1/admin/support/tickets?status=open&priority=high&page=1
Authorization: Bearer {{admin_token}}
```

### 15.2 Update Ticket
```http
PUT {{base_url}}/api/v1/admin/support/tickets/{{ticket_id}}
Authorization: Bearer {{admin_token}}
Content-Type: application/json

{
  "status": "in_progress",
  "priority": "high",
  "assigned_to": "admin_uuid_here"
}
```

### 15.3 Admin Add Message
```http
POST {{base_url}}/api/v1/admin/support/tickets/{{ticket_id}}/messages
Authorization: Bearer {{admin_token}}
Content-Type: application/json

{
  "message": "We are looking into your issue and will resolve it shortly.",
  "internal_note": false
}
```

---

## 📦 16. Bulk Admin APIs

### 16.1 Bulk Verify Drivers
```http
POST {{base_url}}/api/v1/admin/bulk/verify-drivers
Authorization: Bearer {{admin_token}}
Content-Type: application/json

{
  "driver_ids": [
    "driver_uuid_1",
    "driver_uuid_2",
    "driver_uuid_3"
  ],
  "notes": "Batch verification - documents verified"
}
```

### 16.2 Bulk Notify
```http
POST {{base_url}}/api/v1/admin/bulk/notify
Authorization: Bearer {{admin_token}}
Content-Type: application/json

{
  "target_type": "drivers",
  "target_filters": {
    "city": "Mumbai",
    "is_online": true
  },
  "title": "High demand alert!",
  "message": "Andheri area has high demand. Go online to earn more!",
  "channels": ["push", "sms"]
}
```

### 16.3 Bulk Import Drivers
```http
POST {{base_url}}/api/v1/admin/bulk/import-drivers
Authorization: Bearer {{admin_token}}
Content-Type: multipart/form-data

# Upload CSV file with driver data
# Fields: name, phone, license_number, vehicle_type, etc.
```

### 16.4 Bulk Update Driver Status
```http
POST {{base_url}}/api/v1/admin/bulk/update-driver-status
Authorization: Bearer {{admin_token}}
Content-Type: application/json

{
  "driver_ids": [
    "driver_uuid_1",
    "driver_uuid_2"
  ],
  "status": "suspended",
  "reason": "violation_of_terms"
}
```

---

## � 17. Notifications API

### 17.1 Get Notifications (Paginated)
```http
GET {{base_url}}/api/v1/notifications?page=1&limit=20
Authorization: Bearer {{access_token}}

# Response:
{
  "success": true,
  "data": {
    "notifications": [
      {
        "id": "notif_uuid",
        "type": "ride_completed",
        "title": "Ride Completed",
        "body": "Your ride has been completed. Fare: ₹125",
        "status": "unread",
        "data": { "ride_id": "ride_uuid", "fare": 125 },
        "created_at": "2026-05-06T08:30:00Z"
      }
    ],
    "pagination": {
      "page": 1,
      "limit": 20,
      "total": 45,
      "total_pages": 3,
      "has_next": true,
      "has_prev": false
    },
    "unread_count": 3
  }
}
```

### 17.2 Mark Notification as Read
```http
PATCH {{base_url}}/api/v1/notifications/{{notification_id}}/read
Authorization: Bearer {{access_token}}

# Response:
{
  "success": true,
  "message": "Notification marked as read"
}
```

### 17.3 Mark All Notifications as Read
```http
PATCH {{base_url}}/api/v1/notifications/read-all
Authorization: Bearer {{access_token}}

# Response:
{
  "success": true,
  "message": "All notifications marked as read",
  "data": { "marked_count": 3 }
}
```

### 17.4 Delete Notification
```http
DELETE {{base_url}}/api/v1/notifications/{{notification_id}}
Authorization: Bearer {{access_token}}

# Response:
{
  "success": true,
  "message": "Notification deleted"
}
```

### 17.5 Register Device Token (Push Notifications)
```http
POST {{base_url}}/api/v1/notifications/device-token
Authorization: Bearer {{access_token}}
Content-Type: application/json

{
  "token": "fcm_token_here",
  "platform": "android"
}
```

---

## ⚙️ 18. Config API

### 18.1 Get Config (Unified Endpoint - Role-Based)
```http
# Public access - returns public config
GET {{base_url}}/api/v1/config

# Admin access - returns full config with system status
GET {{base_url}}/api/v1/config
Authorization: Bearer {{admin_token}}
```

**Public Response (Unauthenticated):**
```json
{
  "success": true,
  "data": {
    "features": {
      "surge_pricing": true,
      "scheduled_rides": true,
      "wallet_enabled": true,
      "sos_enabled": true,
      "chat_enabled": true,
      "referrals_enabled": true
    },
    "limits": {
      "max_scheduled_rides": 5,
      "max_emergency_contacts": 5,
      "max_saved_addresses": 10,
      "default_search_radius_km": 5
    },
    "timeouts": {
      "driver_search_seconds": 30,
      "ride_request_timeout": 15,
      "otp_expiry_minutes": 5,
      "cancellation_window_min": 5
    },
    "version": "1.0.0",
    "min_app_versions": {
      "ios": "1.0.0",
      "android": "1.0.0"
    }
  }
}
```

**Admin Response (Authenticated with admin role):**
```json
{
  "success": true,
  "data": {
    "public": { ... },
    "system": {
      "database_status": "healthy",
      "redis_status": "healthy",
      "websocket_servers": 1,
      "active_drivers": 150,
      "pending_rides": 12,
      "avg_response_ms": 45,
      "error_rate": 0.001
    }
  }
}
```

### 18.2 Get Cancellation Reasons
```http
GET {{base_url}}/api/v1/config/cancellation-reasons

# Response:
{
  "success": true,
  "data": {
    "reasons": [
      {"code": "rider_cancelled", "label": "Rider cancelled", "applies_to": "rider"},
      {"code": "driver_cancelled", "label": "Driver cancelled", "applies_to": "driver"},
      {"code": "no_driver_found", "label": "No driver found", "applies_to": "system"},
      {"code": "wrong_pickup_location", "label": "Wrong pickup location", "applies_to": "rider"},
      {"code": "pickup_too_far", "label": "Pickup location too far", "applies_to": "driver"},
      {"code": "rider_no_show", "label": "Rider didn't show up", "applies_to": "driver"},
      {"code": "driver_no_show", "label": "Driver didn't show up", "applies_to": "rider"},
      {"code": "vehicle_issue", "label": "Vehicle issue", "applies_to": "driver"},
      {"code": "other", "label": "Other", "applies_to": "all"}
    ]
  }
}
```

### 18.3 Update Config (Admin Only)
```http
PATCH {{base_url}}/api/v1/admin/config
Authorization: Bearer {{admin_token}}
Content-Type: application/json

{
  "key": "features.surge_pricing",
  "value": false
}

# Response:
{
  "success": true,
  "message": "Config updated",
  "data": {
    "key": "features.surge_pricing",
    "value": false
  }
}
```

---

## 🔌 19. WebSocket API (Unified Endpoint)

### 19.1 Unified WebSocket Connection (Production-Ready)
```javascript
// NEW: Unified endpoint with type query parameter
// Supports horizontal scaling with Redis Pub/Sub

// Rider Connection
const ws = new WebSocket('ws://localhost:8080/ws?type=rider&user_id={{user_id}}&token={{access_token}}');

// Driver Connection
const ws = new WebSocket('ws://localhost:8080/ws?type=driver&user_id={{driver_id}}&token={{driver_token}}');

// Admin Connection
const ws = new WebSocket('ws://localhost:8080/ws?type=admin&user_id={{admin_id}}&token={{admin_token}}');

ws.onopen = function() {
  // Subscribe to channels
  ws.send(JSON.stringify({
    type: 'subscribe',
    channel: 'ride:{{ride_id}}'
  }));
};

ws.onmessage = function(event) {
  const data = JSON.parse(event.data);
  console.log('WebSocket message:', data);

  // Handle real-time notifications
  if (data.type === 'notification.new') {
    showNotification(data.payload);
  }
};
```

### 19.2 WebSocket Message Protocol
```javascript
// Client → Server: Location Update (Driver)
{
  "type": "location_update",
  "lat": 19.0760,
  "lng": 72.8777,
  "heading": 90,
  "speed": 25.5
}

// Client → Server: Subscribe to ride
{
  "type": "subscribe",
  "channel": "ride:{{ride_id}}"
}

// Server → Client: Ride Status Update
{
  "type": "ride_status",
  "ride_id": "ride_uuid",
  "status": "driver_arrived",
  "timestamp": 1649251200000
}

// Server → Client: Real-time Notification
{
  "type": "notification.new",
  "payload": {
    "id": "notif_uuid",
    "title": "Ride Completed",
    "body": "Your ride has been completed"
  }
}

// Client → Server: Mark notification read
{
  "type": "notification.read",
  "notification_id": "notif_uuid"
}
```

### 19.3 Production Scaling Notes
- Redis Pub/Sub syncs across multiple server nodes
- Load balancer uses consistent hashing or sticky sessions
- Each node subscribes to Redis channels for broadcast
- Stateless WebSocket servers enable horizontal scaling

---

## 🏥 18. Health Check APIs

### 18.1 Basic Health
```http
GET {{base_url}}/health
```

### 18.2 Detailed Health
```http
GET {{base_url}}/health/detailed
```

### 18.3 Readiness Check
```http
GET {{base_url}}/ready
```

### 18.4 Liveness Check
```http
GET {{base_url}}/live
```

### 18.5 Metrics (Prometheus)
```http
GET {{base_url}}/metrics
```

---

## 📝 Postman Environment Setup

Create a Postman environment with these variables:

| Variable | Initial Value | Description |
|----------|---------------|-------------|
| `base_url` | `http://localhost:8080` | API base URL |
| `access_token` | (empty) | Rider JWT token |
| `driver_token` | (empty) | Driver JWT token |
| `admin_token` | (empty) | Admin JWT token |
| `refresh_token` | (empty) | Refresh token |
| `user_id` | (empty) | Current user ID |
| `ride_id` | (empty) | Current ride ID |
| `driver_id` | (empty) | Current driver ID |
| `payment_method_id` | (empty) | Saved payment method ID |
| `contact_id` | (empty) | Emergency contact ID |
| `ticket_id` | (empty) | Support ticket ID |

---

## 🔄 Testing Flow (Recommended Order)

1. **Auth Flow:**
   - Request OTP → Verify OTP → Get Profile

2. **Rider Flow:**
   - Add Emergency Contact → Add Payment Method → Estimate Fare → Request Ride → Pay for Ride → Rate Driver

3. **Driver Flow:**
   - Register Driver → Go Online → Accept Ride → Start Ride → Complete Ride → Get Earnings

4. **Admin Flow:**
   - Get Dashboard → Verify Driver → Get Reports → Process Withdrawal

5. **Support Flow:**
   - Create Ticket → Add Message → Admin Update Ticket → Admin Resolve

---

## ⚠️ Important Headers

Always include these headers for protected endpoints:
```
Authorization: Bearer {{access_token}}
Content-Type: application/json
```

For idempotent operations, also include:
```
Idempotency-Key: unique_string_here
```

For API versioning (optional):
```
Accept-Version: v1
```

---

**Total APIs: 70+ endpoints covering all features** ✅
