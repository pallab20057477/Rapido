# Rapido Backend API Testing Guide

This is a route-verified, Postman-ready guide for the current backend implementation.
All endpoints and sample payloads in this file are aligned with the active route definitions and controller binding rules.

## Base URL

Use:

```text
http://localhost:8080
```

If your server starts on a fallback port, use that port from startup logs.

## Postman Environment Variables

Create these variables:

- base_url = http://localhost:8080
- access_token = rider or driver JWT token
- refresh_token = refresh token
- admin_token = admin JWT token
- idempotency_key = unique key per request (example: ride-{{uuid}})
- ride_id = UUID of an existing ride
- driver_id = UUID of an existing driver
- payment_id = UUID of an existing payment
- ticket_id = UUID of an existing support ticket
- contact_id = UUID of an existing emergency contact
- withdrawal_id = UUID of an existing withdrawal
- notification_id = UUID of an existing notification
- sos_id = UUID of an SOS event
- rating_id = UUID of a rating record
- surge_id = UUID of a surge pricing record
- payment_method_id = UUID of payment method
- user_id = UUID for websocket query usage
- google_id_token = valid Google ID token

## Common Headers

Protected endpoint:

```text
Authorization: Bearer {{access_token}}
Content-Type: application/json
```

Admin endpoint:

```text
Authorization: Bearer {{admin_token}}
Content-Type: application/json
```

Idempotent endpoint (when required):

```text
Idempotency-Key: {{idempotency_key}}
```

## Critical OTP Notes

- Auth OTP test constant in code is currently 123456.
- Verify OTP endpoint should use otp = "123456" in local/dev test mode.
- Ride start OTP is not fixed. Use the ride_otp generated during ride creation and stored for that ride.

## Authentication APIs

### Request OTP
- Method: POST
- URL: {{base_url}}/api/v1/auth/otp/request
- Auth: none
- Body:

```json
{
  "phone": "9876543211"
}
```

### Verify OTP
- Method: POST
- URL: {{base_url}}/api/v1/auth/otp/verify
- Auth: none
- Body:

```json
{
  "phone": "9876543211",
  "email": "rider@example.com",
  "otp": "123456",
  "name": "Test Rider",
  "user_type": "rider"
}
```

### Password Login
- Method: POST
- URL: {{base_url}}/api/v1/auth/login
- Auth: none
- Body:

```json
{
  "identifier": "rider@example.com",
  "password": "yourpassword123"
}
```

### Refresh Token
- Method: POST
- URL: {{base_url}}/api/v1/auth/refresh
- Auth: none
- Body:

```json
{
  "refresh_token": "{{refresh_token}}"
}
```

### Google Login
- Method: POST
- URL: {{base_url}}/api/v1/auth/google
- Auth: none
- Body:

```json
{
  "id_token": "{{google_id_token}}",
  "phone": "9876543210"
}
```

### Logout
- Method: POST
- URL: {{base_url}}/api/v1/auth/logout
- Auth: Bearer token
- Body:

```json
{
  "refresh_token": "{{refresh_token}}"
}
```

### Get Profile
- Method: GET
- URL: {{base_url}}/api/v1/auth/profile

### Update Profile
- Method: PATCH
- URL: {{base_url}}/api/v1/auth/profile
- Body:

```json
{
  "name": "Updated Rider",
  "email": "updated@example.com",
  "profile_image": "https://example.com/profile.png"
}
```

### Set Password
- Method: POST
- URL: {{base_url}}/api/v1/auth/password/set
- Body:

```json
{
  "password": "yourpassword123"
}
```

### Change Password
- Method: POST
- URL: {{base_url}}/api/v1/auth/password/change
- Body:

```json
{
  "old_password": "yourpassword123",
  "new_password": "newpassword456"
}
```

### Password Status
- Method: GET
- URL: {{base_url}}/api/v1/auth/password/status

## Emergency Contacts and SOS

### Add Emergency Contact
- Method: POST
- URL: {{base_url}}/api/v1/auth/emergency-contacts
- Body:

```json
{
  "name": "Mom",
  "phone": "9999999999",
  "relationship": "parent",
  "priority": 1
}
```

### Get Emergency Contacts
- Method: GET
- URL: {{base_url}}/api/v1/auth/emergency-contacts

### Update Emergency Contact
- Method: PUT
- URL: {{base_url}}/api/v1/auth/emergency-contacts/{{contact_id}}
- Body:

```json
{
  "name": "Dad",
  "phone": "9888888888",
  "relationship": "parent",
  "priority": 2
}
```

### Remove Emergency Contact
- Method: DELETE
- URL: {{base_url}}/api/v1/auth/emergency-contacts/{{contact_id}}

### Trigger SOS
- Method: POST
- URL: {{base_url}}/api/v1/sos/trigger
- Body:

```json
{
  "latitude": 12.9716,
  "longitude": 77.5946,
  "address": "MG Road, Bengaluru",
  "ride_id": "{{ride_id}}"
}
```

### SOS History
- Method: GET
- URL: {{base_url}}/api/v1/sos/history?page=1&per_page=10

### Admin SOS Active
- Method: GET
- URL: {{base_url}}/api/v1/admin/sos/active?page=1&per_page=10

### Admin Resolve SOS
- Method: POST
- URL: {{base_url}}/api/v1/admin/sos/{{sos_id}}/resolve
- Body:

```json
{
  "notes": "Resolved and user contacted"
}
```

## Rating APIs

### Submit Ride Rating
- Method: POST
- URL: {{base_url}}/api/v1/rides/{{ride_id}}/rate
- Body:

```json
{
  "rating": 5,
  "review": "Smooth and safe ride",
  "categories": {
    "cleanliness": 5,
    "punctuality": 5,
    "driving_skill": 5,
    "behavior": 5
  }
}
```

### My Rating for Ride
- Method: GET
- URL: {{base_url}}/api/v1/rides/{{ride_id}}/my-rating

### Driver Reviews
- Method: GET
- URL: {{base_url}}/api/v1/drivers/{{driver_id}}/reviews?page=1&per_page=10

### Driver Rating Summary
- Method: GET
- URL: {{base_url}}/api/v1/drivers/{{driver_id}}/rating-summary

### Report Rating
- Method: POST
- URL: {{base_url}}/api/v1/ratings/{{rating_id}}/report
- Body:

```json
{
  "reason": "abusive_language",
  "details": "Rating contains inappropriate content"
}
```

## Support Ticket APIs

### Create Ticket
- Method: POST
- URL: {{base_url}}/api/v1/users/support/tickets
- Body:

```json
{
  "category": "payment",
  "priority": "high",
  "subject": "Refund pending",
  "description": "Payment was deducted twice",
  "ride_id": "{{ride_id}}"
}
```

### My Tickets
- Method: GET
- URL: {{base_url}}/api/v1/users/support/tickets?page=1&per_page=10

### Ticket Details
- Method: GET
- URL: {{base_url}}/api/v1/users/support/tickets/{{ticket_id}}

### Add Ticket Message
- Method: POST
- URL: {{base_url}}/api/v1/users/support/tickets/{{ticket_id}}/messages
- Body:

```json
{
  "message": "Any update on this ticket?"
}
```

### Admin Ticket List
- Method: GET
- URL: {{base_url}}/api/v1/admin/support/tickets?status=open&category=payment&page=1&per_page=10

### Admin Update Ticket
- Method: PUT
- URL: {{base_url}}/api/v1/admin/support/tickets/{{ticket_id}}
- Body:

```json
{
  "status": "investigating",
  "priority": "high",
  "assigned_to": "admin-user-id",
  "resolution": "Pending review",
  "refund_amount": 100
}
```

### Admin Add Ticket Message
- Method: POST
- URL: {{base_url}}/api/v1/admin/support/tickets/{{ticket_id}}/messages
- Body:

```json
{
  "message": "We are reviewing this now",
  "is_internal": false
}
```

## Payment Method APIs

### Add Card
- Method: POST
- URL: {{base_url}}/api/v1/payments/methods/card
- Body:

```json
{
  "card_number": "4111111111111111",
  "expiry_month": 12,
  "expiry_year": 2030,
  "cvv": "123",
  "cardholder_name": "Test Rider",
  "card_type": "credit",
  "nickname": "Main Card",
  "set_as_default": true,
  "billing_address": "Bengaluru"
}
```

### Add UPI
- Method: POST
- URL: {{base_url}}/api/v1/payments/methods/upi
- Body:

```json
{
  "vpa": "test@upi",
  "nickname": "Personal UPI",
  "set_as_default": true
}
```

### Get Payment Methods
- Method: GET
- URL: {{base_url}}/api/v1/payments/methods

### Remove Payment Method
- Method: DELETE
- URL: {{base_url}}/api/v1/payments/methods/{{payment_method_id}}

### Set Default Payment Method
- Method: POST
- URL: {{base_url}}/api/v1/payments/methods/{{payment_method_id}}/default

## Driver APIs

### Register Driver
- Method: POST
- URL: {{base_url}}/api/v1/drivers/register
- Body:

```json
{
  "license_number": "KA0120241234567",
  "license_image": "https://example.com/license.png",
  "license_expiry": "2030-12-31T00:00:00Z",
  "rc_number": "KA01RC1234",
  "rc_image": "https://example.com/rc.png",
  "aadhaar_number": "123412341234",
  "aadhaar_image": "https://example.com/aadhaar.png",
  "vehicle_type": "car_go",
  "vehicle_make": "Toyota",
  "vehicle_model": "Etios",
  "vehicle_year": 2021,
  "vehicle_color": "White",
  "vehicle_number_plate": "KA01AB1234",
  "fuel_type": "petrol",
  "vehicle_image": "https://example.com/car.png",
  "languages": ["en", "kn"]
}
```

### Driver Profile
- GET {{base_url}}/api/v1/drivers/profile
- PATCH {{base_url}}/api/v1/drivers/profile

Body example for PATCH:

```json
{
  "languages": ["en", "hi"]
}
```

### Go Online
- Method: POST
- URL: {{base_url}}/api/v1/drivers/online
- Body:

```json
{
  "lat": 12.9716,
  "lng": 77.5946
}
```

### Go Offline
- Method: POST
- URL: {{base_url}}/api/v1/drivers/offline

### Update Driver Location
- Method: POST
- URL: {{base_url}}/api/v1/drivers/location
- Body:

```json
{
  "lat": 12.972,
  "lng": 77.595,
  "accuracy": 5
}
```

### Driver Earnings
- Method: GET
- URL: {{base_url}}/api/v1/drivers/earnings

### Driver Stats
- Method: GET
- URL: {{base_url}}/api/v1/drivers/stats

## Admin Driver Verification Workflow

### List Pending Drivers
- Method: GET
- URL: {{base_url}}/api/v1/admin/drivers/pending?page=1&per_page=10

### Get Driver Details
- Method: GET
- URL: {{base_url}}/api/v1/admin/drivers/{{driver_id}}

### Approve or Reject Driver
- Method: POST
- URL: {{base_url}}/api/v1/admin/drivers/{{driver_id}}/verify
- Body (preferred):

```json
{
  "verified": true,
  "notes": "Documents verified"
}
```

Alternative body:

```json
{
  "action": "approve",
  "notes": "Approved by admin"
}
```

### Admin Create Driver
- Method: POST
- URL: {{base_url}}/api/v1/admin/drivers/create
- Body:

```json
{
  "name": "John Driver",
  "email": "john.driver@example.com",
  "phone": "9876543212",
  "password": "driver123",
  "license_number": "KA0120241234567",
  "license_image": "https://example.com/license.png",
  "license_expiry": "2030-12-31T00:00:00Z",
  "rc_number": "KA01RC1234",
  "rc_image": "https://example.com/rc.png",
  "aadhaar_number": "123412341234",
  "aadhaar_image": "https://example.com/aadhaar.png",
  "vehicle_type": "car_go",
  "vehicle_make": "Toyota",
  "vehicle_model": "Etios",
  "vehicle_year": 2021,
  "vehicle_color": "White",
  "vehicle_number_plate": "KA01AB1234",
  "fuel_type": "petrol",
  "vehicle_image": "https://example.com/car.png",
  "languages": ["en", "hi"],
  "auto_verify": true
}
```

## Ride APIs

### Request Ride
- Method: POST
- URL: {{base_url}}/api/v1/rides
- Headers: Idempotency-Key
- Body:

```json
{
  "vehicle_type": "car_go",
  "pickup_lat": 12.9716,
  "pickup_lng": 77.5946,
  "pickup_address": "MG Road, Bengaluru",
  "dropoff_lat": 12.9352,
  "dropoff_lng": 77.6245,
  "dropoff_address": "Koramangala, Bengaluru",
  "promo_code": "WELCOME10",
  "payment_method": "wallet",
  "preferences": {
    "ac_required": true,
    "female_driver_only": false,
    "luggage_space": true,
    "silence_mode": false,
    "music": true
  }
}
```

### Active Ride
- GET {{base_url}}/api/v1/rides/active

### Ride History
- GET {{base_url}}/api/v1/rides/history?page=1&per_page=10

### Ride by ID
- GET {{base_url}}/api/v1/rides/{{ride_id}}

### Track Ride
- GET {{base_url}}/api/v1/rides/{{ride_id}}/track

### Ride ETA
- GET {{base_url}}/api/v1/rides/{{ride_id}}/eta

### Fare Breakdown
- GET {{base_url}}/api/v1/rides/{{ride_id}}/fare

### Fare Estimate
- GET {{base_url}}/api/v1/rides/estimate?pickup_lat=12.9716&pickup_lng=77.5946&dropoff_lat=12.9352&dropoff_lng=77.6245&vehicle_type=car_go

### Nearby Drivers
- GET {{base_url}}/api/v1/drivers/nearby?lat=12.9716&lng=77.5946&vehicle_type=car_go

### Cancel Ride
- Method: POST
- URL: {{base_url}}/api/v1/rides/{{ride_id}}/cancel
- Headers: Idempotency-Key
- Body:

```json
{
  "reason": "change_of_plan"
}
```

### Apply Promo
- Method: POST
- URL: {{base_url}}/api/v1/rides/{{ride_id}}/apply-promo
- Body:

```json
{
  "promo_code": "WELCOME10"
}
```

### Retry Match
- Method: POST
- URL: {{base_url}}/api/v1/rides/{{ride_id}}/retry
- Headers: Idempotency-Key

### Driver Accept
- POST {{base_url}}/api/v1/rides/{{ride_id}}/accept

### Driver Reject
- POST {{base_url}}/api/v1/rides/{{ride_id}}/reject

### Driver Arrived
- POST {{base_url}}/api/v1/rides/{{ride_id}}/arrived

### Start Ride
- Method: POST
- URL: {{base_url}}/api/v1/rides/{{ride_id}}/start
- Body:

```json
{
  "otp": "<ride_otp_from_ride_record_or_response>"
}
```

### Complete Ride
- Method: POST
- URL: {{base_url}}/api/v1/rides/{{ride_id}}/complete
- Body:

```json
{
  "final_lat": 12.9352,
  "final_lng": 77.6245
}
```

### Update Ride Status
- Method: PATCH
- URL: {{base_url}}/api/v1/rides/{{ride_id}}/status
- Body examples:

```json
{
  "status": "accepted"
}
```

```json
{
  "status": "started",
  "otp": "<ride_otp>"
}
```

```json
{
  "status": "completed",
  "final_lat": 12.9352,
  "final_lng": 77.6245
}
```

```json
{
  "status": "cancelled"
}
```

### Reassign Ride (Admin)
- Method: POST
- URL: {{base_url}}/api/v1/rides/{{ride_id}}/reassign
- Headers: Idempotency-Key

### Match Status (Admin)
- GET {{base_url}}/api/v1/rides/{{ride_id}}/match-status

### Failure Reason (Admin)
- GET {{base_url}}/api/v1/rides/{{ride_id}}/failure-reason

### Cancellation Reasons Config
- GET {{base_url}}/api/v1/config/cancellation-reasons

## Scheduled Ride APIs

### Schedule Ride
- Method: POST
- URL: {{base_url}}/api/v1/rides/schedule
- Headers: Idempotency-Key
- Body:

```json
{
  "vehicle_type": "car_go",
  "pickup_lat": 12.9716,
  "pickup_lng": 77.5946,
  "pickup_address": "MG Road, Bengaluru",
  "dropoff_lat": 12.9352,
  "dropoff_lng": 77.6245,
  "dropoff_address": "Koramangala, Bengaluru",
  "scheduled_at": "2026-12-10T10:30:00Z",
  "notes": "Airport drop",
  "preferences": {
    "female_driver_only": false,
    "ac_required": true,
    "luggage_space": true
  }
}
```

### Get Scheduled Rides
- GET {{base_url}}/api/v1/rides/scheduled

### Get Scheduled Ride Details
- GET {{base_url}}/api/v1/rides/scheduled/{{ride_id}}

### Update Scheduled Ride
- PUT {{base_url}}/api/v1/rides/scheduled/{{ride_id}}
- Headers: Idempotency-Key
- Body: same as schedule ride body

### Cancel Scheduled Ride
- POST {{base_url}}/api/v1/rides/scheduled/{{ride_id}}/cancel
- Headers: Idempotency-Key

## Wallet and Payment APIs

### Get Wallet
- GET {{base_url}}/api/v1/wallet

### Add Money
- POST {{base_url}}/api/v1/wallet/add-money
- Headers: Idempotency-Key
- Body:

```json
{
  "amount": 500,
  "method": "upi"
}
```

### Transactions
- GET {{base_url}}/api/v1/transactions?page=1&per_page=10

### Request Withdrawal
- POST {{base_url}}/api/v1/withdrawals
- Headers: Idempotency-Key
- Body:

```json
{
  "amount": 1000,
  "method": "bank_transfer",
  "bank_details": {
    "account_number": "1234567890",
    "ifsc": "SBIN0001234",
    "account_holder_name": "Driver Test"
  }
}
```

### Process Ride Payment
- POST {{base_url}}/api/v1/payments/rides/{{ride_id}}/pay
- Headers: Idempotency-Key
- Body:

```json
{
  "method": "wallet"
}
```

### Retry Ride Payment
- POST {{base_url}}/api/v1/payments/rides/{{ride_id}}/retry
- Headers: Idempotency-Key
- Body (optional):

```json
{
  "method": "wallet"
}
```

### Payment Status by Ride
- GET {{base_url}}/api/v1/payments/rides/{{ride_id}}

### Refund Payment
- POST {{base_url}}/api/v1/payments/{{payment_id}}/refund
- Headers: Idempotency-Key
- Body:

```json
{
  "amount": 50,
  "reason": "partial refund"
}
```

### Payment Webhook
- Method: POST
- URL: {{base_url}}/api/v1/payments/webhook
- Current routing note: this endpoint is under protected auth group and also checks webhook signature middleware.
- Required headers:
  - Authorization: Bearer {{access_token}} (current route behavior)
  - X-Razorpay-Signature or Stripe-Signature
- Body: raw provider JSON payload

## Admin APIs

### Admin Login
- Method: POST
- URL: {{base_url}}/api/v1/auth/login
- Body:

```json
{
  "identifier": "admin@rapido.com",
  "password": "admin123"
}
```

### Reset Admin Password from ENV
- POST {{base_url}}/api/v1/admin/reset-admin-password

### Admin Debug Password
- GET {{base_url}}/api/v1/admin/debug/password

### Dashboard
- GET {{base_url}}/api/v1/admin/dashboard

### All Rides
- GET {{base_url}}/api/v1/admin/rides?page=1&per_page=20&status=completed

### All Users
- GET {{base_url}}/api/v1/admin/users?page=1&per_page=20

### All Drivers
- GET {{base_url}}/api/v1/admin/drivers?page=1&per_page=20&verified=true

### All Payments
- GET {{base_url}}/api/v1/admin/payments?page=1&per_page=20

### Pending Withdrawals
- GET {{base_url}}/api/v1/admin/withdrawals/pending?page=1&per_page=20

### Process Withdrawal
- POST {{base_url}}/api/v1/admin/withdrawals/process
- Body:

```json
{
  "withdrawal_id": "{{withdrawal_id}}",
  "approved": true,
  "rejection_reason": ""
}
```

### Create Surge Pricing
- POST {{base_url}}/api/v1/admin/surge-pricing
- Body:

```json
{
  "area_name": "Bangalore CBD",
  "lat": 12.9716,
  "lng": 77.5946,
  "radius_km": 4,
  "multiplier": 1.5,
  "reason": "peak demand",
  "duration_hours": 2
}
```

### Remove Surge Pricing
- DELETE {{base_url}}/api/v1/admin/surge-pricing/{{surge_id}}

### Create Promo Code
- POST {{base_url}}/api/v1/admin/promo-codes
- Body:

```json
{
  "code": "WELCOME10",
  "description": "Welcome discount",
  "discount_type": "percentage",
  "discount_value": 10,
  "max_discount": 100,
  "min_ride_amount": 50,
  "max_uses": 1000,
  "max_uses_per_user": 3,
  "vehicle_types": ["car_go", "car_x"],
  "start_date": "2026-01-01T00:00:00Z",
  "end_date": "2026-12-31T23:59:59Z"
}
```

### Reports
- GET {{base_url}}/api/v1/admin/reports?type=daily_earnings
- Valid types: daily_earnings, driver_performance, ride_funnel, peak_hours, revenue_summary

### Ledger Accounts
- GET {{base_url}}/api/v1/admin/ledger/accounts

### Ledger Entries
- GET {{base_url}}/api/v1/admin/ledger/entries

### Ledger Audit Batch
- POST {{base_url}}/api/v1/admin/ledger/audit-batch
- Body:

```json
{
  "batch_id": "batch-20260514-001"
}
```

### Ledger Account Balance
- GET {{base_url}}/api/v1/admin/ledger/account-balance

### Update App Config
- PATCH {{base_url}}/api/v1/admin/config
- Body:

```json
{
  "key": "max_active_rides_per_driver",
  "value": 2
}
```

## Bulk Admin APIs

### Bulk Verify Drivers
- POST {{base_url}}/api/v1/admin/bulk/verify-drivers
- Body:

```json
{
  "driver_ids": ["{{driver_id}}"],
  "notes": "Bulk approval"
}
```

### Bulk Notify
- POST {{base_url}}/api/v1/admin/bulk/notify
- Body:

```json
{
  "user_ids": ["{{user_id}}"],
  "user_type": "rider",
  "title": "Service Update",
  "body": "New fare offer available",
  "channels": ["push", "sms"]
}
```

### Bulk Import Drivers
- POST {{base_url}}/api/v1/admin/bulk/import-drivers
- Body (minimum example):

```json
{
  "drivers": [
    {
      "name": "Imported Driver",
      "email": "imported.driver@example.com",
      "phone": "9876500000",
      "license_number": "KA0120247654321",
      "rc_number": "KA01RC5678",
      "vehicle_type": "car_go",
      "vehicle_number_plate": "KA01CD5678"
    }
  ]
}
```

### Bulk Update Driver Status
- POST {{base_url}}/api/v1/admin/bulk/update-driver-status
- Body:

```json
{
  "driver_ids": ["{{driver_id}}"],
  "status": "inactive",
  "reason": "Temporary suspension"
}
```

## Notifications

### Get Notifications
- GET {{base_url}}/api/v1/notifications?page=1&limit=20

### Mark All as Read
- PATCH {{base_url}}/api/v1/notifications/read-all

### Mark One as Read
- PATCH {{base_url}}/api/v1/notifications/{{notification_id}}/read

### Delete Notification
- DELETE {{base_url}}/api/v1/notifications/{{notification_id}}

Note: current implementation marks as read and returns success, rather than hard deleting row content.

## Config and WebSocket

### Unified Config
- GET {{base_url}}/api/v1/config
- Works with optional auth token.

### WebSocket
- GET {{base_url}}/ws?type=rider&user_id={{user_id}}
- Header: Authorization: Bearer {{access_token}}

## Health and Metrics

- GET {{base_url}}/health
- GET {{base_url}}/health/detailed
- GET {{base_url}}/ready
- GET {{base_url}}/live
- GET {{base_url}}/metrics

## CRM Webhook

### CRM Webhook Endpoint
- Method: POST
- URL: {{base_url}}/api/v1/webhooks/crm
- Auth: none
- Required headers usually include:
  - X-API-Key or X-Rapido-Webhook-Key
  - X-Webhook-Timestamp
  - X-Webhook-ID or X-Event-ID
  - X-Webhook-Signature or X-Signature
- Body: raw CRM event JSON

## Recommended End-to-End Test Order

1. Auth: request OTP, verify OTP, refresh token.
2. Profile and password setup.
3. Emergency contacts and SOS.
4. Driver registration and admin verification.
5. Rider request ride and driver flow: accept, arrived, start, complete.
6. Payment flow: pay, retry, refund, wallet checks.
7. Support and rating flows.
8. Admin dashboard, reports, bulk operations.
9. Notifications, config, websocket, health endpoints.

## Practical Testing Notes

- Always use a fresh Idempotency-Key for a new create/cancel/pay operation.
- Reusing the same key should replay stored idempotent response.
- Ensure DB and Redis are running before ride/payment/websocket testing.
- For strict signature-verified webhooks, send raw JSON body, not form-data.
