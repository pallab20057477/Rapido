# Rapido Backend Postman Production Test Guide

This guide is for validating the current backend safely in Postman before production launch. It is aligned with the active route table and is focused on end-to-end verification, with extra attention on location-related flows.

Use the JSON bodies below directly in Postman for every request that accepts JSON. Every POST, PATCH, and PUT route in the active route table is covered. For GET and DELETE requests, use the URL examples and query parameters shown.

## Goal

Use the existing Postman collection to confirm that:
- routes are registered correctly
- auth works for rider, driver, and admin roles
- ride lifecycle works end to end
- location-based endpoints behave correctly
- bulk admin actions are restricted and safe
- health and readiness endpoints respond correctly

## Important

Do not use this guide as a runtime dependency. It is only for testing and pre-deploy verification.

## Recommended Postman Setup

Import the collection from:
- `postman/Rapido_Complete_API_Collection.json`

Create a Postman environment with these variables:

- `base_url` = `http://localhost:8080`
- `rider_phone` = test rider phone number
- `driver_phone` = test driver phone number
- `admin_email` = admin email
- `admin_password` = admin password
- `access_token` = rider JWT
- `driver_token` = driver JWT
- `admin_token` = admin JWT
- `refresh_token` = refresh token
- `rider_id` = rider UUID
- `driver_id` = driver UUID
- `ride_id` = ride UUID
- `scheduled_ride_id` = scheduled ride UUID
- `payment_id` = payment UUID
- `payment_method_id` = payment method UUID
- `contact_id` = emergency contact UUID
- `ticket_id` = support ticket UUID
- `notification_id` = notification UUID
- `sos_id` = SOS UUID
- `withdrawal_id` = withdrawal UUID
- `surge_id` = surge pricing UUID
- `rating_id` = rating UUID
- `ride_otp` = ride OTP returned by the backend
- `idempotency_key` = a unique key for mutation tests

## Header Rules

Use these headers where required:

```http
Authorization: Bearer {{access_token}}
Content-Type: application/json
```

For driver requests:

```http
Authorization: Bearer {{driver_token}}
Content-Type: application/json
```

For admin requests:

```http
Authorization: Bearer {{admin_token}}
Content-Type: application/json
```

For idempotent actions:

```http
Idempotency-Key: {{idempotency_key}}
```

## Suggested Test Order

Run the folders in this order so the later requests have the IDs they need:

1. Health checks
2. Admin login
3. Rider auth
4. Driver auth and registration
5. Driver location and availability
6. Ride estimate and request
7. Driver ride actions
8. Ride tracking and history
9. Payments and wallet
10. Support, SOS, ratings, notifications
11. Admin approvals and reports
12. Bulk admin operations

## JSON Request Templates

### 1. Health Checks

Health checks do not require JSON bodies.

### 2. Authentication

#### Request OTP

```json
{
  "phone": "9876543210"
}
```

#### Verify OTP for rider

```json
{
  "phone": "9876543210",
  "email": "rider@example.com",
  "otp": "123456",
  "name": "Test Rider",
  "user_type": "rider"
}
```

#### Verify OTP for driver

```json
{
  "phone": "9876543211",
  "email": "driver@example.com",
  "otp": "123456",
  "name": "Test Driver",
  "user_type": "driver"
}
```

#### Password login

```json
{
  "identifier": "admin@rapido.com",
  "password": "admin123"
}
```

#### Refresh token

```json
{
  "refresh_token": "{{refresh_token}}"
}
```

#### Google login

```json
{
  "id_token": "google_id_token_here",
  "phone": "9876543210"
}
```

#### Logout

```json
{
  "refresh_token": "{{refresh_token}}"
}
```

### 3. Rider Profile and Emergency Contacts

#### Update profile

```json
{
  "name": "Updated Rider",
  "email": "updated@example.com",
  "profile_image": "https://example.com/profile.png"
}
```

#### Set password

```json
{
  "password": "yourpassword123"
}
```

#### Change password

```json
{
  "old_password": "yourpassword123",
  "new_password": "newpassword456"
}
```

#### Add emergency contact

```json
{
  "name": "Mom",
  "phone": "9999999999",
  "relationship": "parent",
  "priority": 1
}
```

#### Update emergency contact

```json
{
  "name": "Dad",
  "phone": "9888888888",
  "relationship": "parent",
  "priority": 2
}
```

#### Trigger SOS

```json
{
  "latitude": 12.9716,
  "longitude": 77.5946,
  "address": "MG Road, Bengaluru",
  "ride_id": "{{ride_id}}"
}
```

### 4. Driver Onboarding and Live Location

#### Register driver

```json
{
  "license_number": "KA01DL1234",
  "license_image": "https://example.com/license.jpg",
  "license_expiry": "2027-12-31T00:00:00Z",
  "rc_number": "KA01RC1234",
  "rc_image": "https://example.com/rc.jpg",
  "aadhaar_number": "123412341234",
  "aadhaar_image": "https://example.com/aadhaar.jpg",
  "vehicle_type": "car",
  "vehicle_make": "Toyota",
  "vehicle_model": "Etios",
  "vehicle_year": 2022,
  "vehicle_color": "White",
  "vehicle_number_plate": "KA01AB1234",
  "fuel_type": "petrol",
  "vehicle_image": "https://example.com/vehicle.jpg",
  "languages": ["English", "Kannada"]
}
```

#### Update driver profile

```json
{
  "languages": ["English", "Kannada", "Hindi"]
}
```

#### Driver goes online

```json
{
  "lat": 12.9716,
  "lng": 77.5946
}
```

#### Driver updates live location

```json
{
  "lat": 12.972,
  "lng": 77.5955,
  "accuracy": 12.5
}
```

### 5. Ride Estimation and Ride Request

#### Estimate fare

Use query parameters:

```text
{{base_url}}/api/v1/rides/estimate?pickup_lat=12.9716&pickup_lng=77.5946&dropoff_lat=12.9784&dropoff_lng=77.6408&vehicle_type=car
```

#### Request ride

```json
{
  "vehicle_type": "car",
  "pickup_lat": 12.9716,
  "pickup_lng": 77.5946,
  "pickup_address": "MG Road, Bengaluru",
  "dropoff_lat": 12.9784,
  "dropoff_lng": 77.6408,
  "dropoff_address": "Indiranagar, Bengaluru",
  "promo_code": "",
  "payment_method": "cash",
  "preferences": {
    "female_driver_only": false,
    "ac_required": true,
    "silence_mode": false,
    "music": false,
    "luggage_space": true
  }
}
```

#### Cancel ride

```json
{
  "reason": "change_of_plans"
}
```

#### Retry matching

```json
{}
```

#### Apply promo code

```json
{
  "promo_code": "RAPIDO50"
}
```

### 6. Driver Ride Actions

#### Start ride

```json
{
  "otp": "{{ride_otp}}"
}
```

#### Update ride status

```json
{
  "status": "started",
  "otp": "{{ride_otp}}",
  "final_lat": 12.9721,
  "final_lng": 77.5959
}
```

#### Mark ride complete

```json
{
  "final_lat": 12.9784,
  "final_lng": 77.6408
}
```

#### Reassign ride

```json
{
  "reason": "driver_unavailable",
  "preferred_driver_types": ["car", "suv"],
  "priority": "high"
}
```

### 7. Payments and Wallet

#### Add money to wallet

```json
{
  "amount": 500,
  "method": "upi"
}
```

#### Request withdrawal

```json
{
  "amount": 250,
  "method": "bank_transfer",
  "bank_details": {
    "account_name": "Test Rider",
    "account_number": "1234567890",
    "ifsc": "HDFC0001234",
    "bank_name": "HDFC Bank"
  }
}
```

#### Pay for ride

```json
{
  "method": "upi"
}
```

#### Add card payment method

```json
{
  "card_number": "4111111111111111",
  "expiry_month": 12,
  "expiry_year": 2028,
  "cvv": "123",
  "cardholder_name": "Test Rider",
  "card_type": "debit",
  "nickname": "Primary card",
  "set_as_default": true,
  "billing_address": "123 MG Road, Bengaluru"
}
```

#### Add UPI payment method

```json
{
  "vpa": "testuser@upi",
  "nickname": "Primary UPI",
  "set_as_default": true
}
```

#### Refund payment

```json
{
  "reason": "duplicate_payment",
  "amount": 100
}
```

### 8. Support Tickets and Ratings

#### Create support ticket

```json
{
  "category": "payment",
  "priority": "high",
  "subject": "Refund pending",
  "description": "Payment was deducted twice",
  "ride_id": "{{ride_id}}"
}
```

#### Add support message

```json
{
  "message": "Please check the refund status"
}
```

#### Submit ride rating

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

#### Report rating

```json
{
  "reason": "abusive_language",
  "details": "Rating contains inappropriate content"
}
```

### 9. Admin Actions

#### Create driver as admin

```json
{
  "name": "New Driver",
  "email": "newdriver@example.com",
  "phone": "9876543222",
  "password": "driverpass123",
  "license_number": "KA01DL5678",
  "license_image": "https://example.com/license.jpg",
  "license_expiry": "2027-12-31T00:00:00Z",
  "rc_number": "KA01RC5678",
  "rc_image": "https://example.com/rc.jpg",
  "aadhaar_number": "567856785678",
  "aadhaar_image": "https://example.com/aadhaar.jpg",
  "languages": ["English", "Kannada"],
  "vehicle_type": "car",
  "vehicle_make": "Honda",
  "vehicle_model": "City",
  "vehicle_year": 2021,
  "vehicle_color": "Black",
  "vehicle_number_plate": "KA01CD5678",
  "fuel_type": "petrol",
  "vehicle_image": "https://example.com/vehicle.jpg",
  "auto_verify": false
}
```

#### Verify driver

```json
{
  "verified": true,
  "notes": "Documents checked and approved"
}
```

#### Process withdrawal

```json
{
  "withdrawal_id": "{{withdrawal_id}}",
  "approved": true,
  "rejection_reason": ""
}
```

#### Create surge pricing

```json
{
  "area_name": "Bengaluru East",
  "lat": 12.9716,
  "lng": 77.5946,
  "radius_km": 5,
  "multiplier": 1.5,
  "reason": "Peak demand",
  "duration_hours": 2
}
```

#### Create promo code

```json
{
  "code": "WELCOME50",
  "description": "50 percent off for new users",
  "discount_type": "percentage",
  "discount_value": 50,
  "max_discount": 100,
  "min_ride_amount": 200,
  "max_uses": 500,
  "max_uses_per_user": 1,
  "vehicle_types": ["car", "bike"],
  "start_date": "2026-05-14T00:00:00Z",
  "end_date": "2026-06-14T00:00:00Z"
}
```

#### Update app config

```json
{
  "key": "app_name",
  "value": "Rapido Production"
}
```

### 10. Bulk Admin Actions

#### Verify drivers in bulk

```json
{
  "driver_ids": [
    "11111111-1111-1111-1111-111111111111",
    "22222222-2222-2222-2222-222222222222"
  ],
  "notes": "Batch verification from onboarding queue"
}
```

#### Bulk notify

```json
{
  "user_ids": [
    "33333333-3333-3333-3333-333333333333",
    "44444444-4444-4444-4444-444444444444"
  ],
  "user_type": "driver",
  "title": "Maintenance Window",
  "body": "The app will be briefly unavailable for maintenance.",
  "channels": ["push", "sms"]
}
```

#### Bulk import drivers

```json
{
  "drivers": [
    {
      "name": "Driver One",
      "phone": "9876543301",
      "email": "driver1@example.com",
      "city": "Bengaluru",
      "vehicle_type": "car",
      "vehicle_number": "KA01AA0001"
    },
    {
      "name": "Driver Two",
      "phone": "9876543302",
      "email": "driver2@example.com",
      "city": "Bengaluru",
      "vehicle_type": "bike",
      "vehicle_number": "KA01BB0002"
    }
  ]
}
```

#### Bulk update driver status

```json
{
  "driver_ids": [
    "11111111-1111-1111-1111-111111111111",
    "22222222-2222-2222-2222-222222222222"
  ],
  "status": "inactive",
  "reason": "Compliance review"
}
```

## 1. Health Checks

These should work before anything else.

### Test
- `GET /health`
- `GET /health/detailed`
- `GET /ready`
- `GET /live`
- `GET /metrics`

### Expected
- `GET /health` and `GET /live` should return `200`
- `GET /ready` may return `200` or `503` depending on DB/Redis state
- `GET /health/detailed` should show service status, especially `database` and `redis`

## 2. Authentication

### Rider OTP flow
1. Request OTP
2. Verify OTP
3. Store `access_token` and `refresh_token`

### Driver login or OTP flow
1. Request OTP for the driver phone
2. Verify OTP
3. If driver profile exists, use the driver token
4. If not, register the driver first

### Admin login
Use password login for the configured admin account.

### Expected
- login and OTP verify should return JWT tokens
- refresh token should issue a new access token
- logout should revoke session/token state if configured

## 3. Rider Profile and Account

Test these with `{{access_token}}`:

- `GET /api/v1/auth/profile`
- `PATCH /api/v1/auth/profile`
- `POST /api/v1/auth/password/set`
- `POST /api/v1/auth/password/change`
- `GET /api/v1/auth/password/status`
- emergency contact CRUD

### Expected
- `GET` returns rider profile
- `PATCH` updates only allowed profile fields
- password actions must fail with clear validation if inputs are wrong

## 4. Driver Onboarding

Test these with `{{driver_token}}` or a newly verified driver account:

- `POST /api/v1/drivers/register`
- `GET /api/v1/drivers/profile`
- `PATCH /api/v1/drivers/profile`
- `GET /api/v1/drivers/earnings`
- `GET /api/v1/drivers/stats`

### Expected
- registration creates driver data and vehicle data
- profile read returns the current driver profile
- update profile changes only editable fields

## 5. Location-Related Testing

These are the most important tests if you want to confirm the backend is working correctly in a real map flow.

### 5.1 Estimate fare
Request:
- `GET /api/v1/rides/estimate?pickup_lat=...&pickup_lng=...&dropoff_lat=...&dropoff_lng=...&vehicle_type=...`

Sample coordinates:
- Pickup: Bengaluru city center `12.9716, 77.5946`
- Dropoff: Indiranagar `12.9784, 77.6408`

### Expected
- returns distance, duration, and fare breakdown
- if Google route data is unavailable, fallback distance should still work
- response should include estimated fare fields and surge data when applicable

### 5.2 Nearby drivers
Request:
- `GET /api/v1/drivers/nearby?lat=12.9716&lng=77.5946&vehicle_type=car`

### Expected
- returns a list of nearby drivers
- each item should include driver id and coordinates
- if no drivers are nearby, it should return an empty list, not an error

### 5.3 Driver goes online
Request body:

```json
{
  "lat": 12.9716,
  "lng": 77.5946
}
```

### Expected
- driver status changes to online
- location is stored and visible to tracking and matching
- if the driver is not verified, the request should fail cleanly

### 5.4 Driver location updates
Request body:

```json
{
  "lat": 12.9720,
  "lng": 77.5955,
  "accuracy": 12.5
}
```

### Expected
- location updates without changing the full profile
- repeated location updates should be safe and should not create broken state
- if the driver is online, the live location should move in matching/tracking flows

### 5.5 Ride request with coordinates
Request body:

```json
{
  "vehicle_type": "car",
  "pickup_lat": 12.9716,
  "pickup_lng": 77.5946,
  "pickup_address": "MG Road, Bengaluru",
  "dropoff_lat": 12.9784,
  "dropoff_lng": 77.6408,
  "dropoff_address": "Indiranagar, Bengaluru",
  "payment_method": "cash",
  "promo_code": "",
  "preferences": {
    "female_driver_only": false,
    "ac_required": true,
    "silence_mode": false,
    "music": false,
    "luggage_space": true
  }
}
```

### Expected
- ride is created successfully
- response contains a ride id and estimated fare
- ride OTP is generated and stored for start verification
- nearby driver matching should start after creation

### 5.6 Track ride
Request:
- `GET /api/v1/rides/{{ride_id}}/track`

### Expected
- rider can see ride state
- if driver location exists, it should be included in the response
- unauthorized users should not be able to view another ride

### 5.7 Ride ETA
Request:
- `GET /api/v1/rides/{{ride_id}}/eta`

### Expected
- ETA should change with ride status and driver location availability

### 5.8 Ride location-related debug checks
These are optional but useful during testing:
- `GET /api/v1/rides/{{ride_id}}`
- `GET /api/v1/rides/{{ride_id}}/fare`
- `GET /api/v1/rides/history`
- `GET /api/v1/drivers/profile`

## 6. Ride Lifecycle End to End

Use this sequence in Postman to confirm the full ride path works:

1. Request OTP for rider
2. Verify OTP and save `{{access_token}}`
3. Request OTP for driver
4. Verify OTP and save `{{driver_token}}`
5. Register the driver if needed
6. Bring the driver online with coordinates
7. Request a ride with pickup and dropoff coordinates
8. Save `{{ride_id}}` from the create response
9. Driver accepts the ride
10. Driver arrives
11. Driver starts the ride using the ride OTP
12. Driver updates ride location if needed
13. Driver completes the ride
14. Check rider history and active ride
15. Verify fare breakdown and payment state

### Expected
- every transition should follow the ride status rules
- invalid transitions should fail with a clear validation error
- idempotent route retries should not duplicate the action

## 7. Rider Actions on Rides

Test these with rider auth:

- `POST /api/v1/rides`
- `GET /api/v1/rides/active`
- `GET /api/v1/rides/history`
- `GET /api/v1/rides/{{ride_id}}`
- `GET /api/v1/rides/{{ride_id}}/track`
- `GET /api/v1/rides/{{ride_id}}/eta`
- `GET /api/v1/rides/{{ride_id}}/fare`
- `POST /api/v1/rides/{{ride_id}}/cancel`
- `POST /api/v1/rides/{{ride_id}}/retry`
- `POST /api/v1/rides/{{ride_id}}/apply-promo`

### Expected
- cancel works only in allowed states
- retry only works when no driver was found
- promo code application should use real promo records from the database

## 8. Driver Ride Actions

Test these with driver auth:

- `POST /api/v1/rides/{{ride_id}}/accept`
- `POST /api/v1/rides/{{ride_id}}/reject`
- `POST /api/v1/rides/{{ride_id}}/arrived`
- `POST /api/v1/rides/{{ride_id}}/start`
- `POST /api/v1/rides/{{ride_id}}/complete`
- `PATCH /api/v1/rides/{{ride_id}}/status`
- `POST /api/v1/rides/{{ride_id}}/reassign`

### Expected
- accept should lock the ride to one driver only
- start requires the correct ride OTP
- complete should calculate fare and close the ride cleanly
- status transitions should reject invalid state changes

## 9. Payments and Wallet

Test these with rider auth:

- `GET /api/v1/wallet`
- `POST /api/v1/wallet/add-money`
- `GET /api/v1/transactions`
- `POST /api/v1/payments/rides/{{ride_id}}/pay`
- `POST /api/v1/payments/rides/{{ride_id}}/retry`
- `GET /api/v1/payments/rides/{{ride_id}}`
- `POST /api/v1/payments/:id/refund`
- `POST /api/v1/withdrawals`

### Expected
- payment and refund routes should be protected with idempotency
- invalid payment states should fail cleanly

## 10. Support, SOS, Notifications, Ratings

Test these with rider auth:

- support ticket create/list/detail/message
- SOS trigger and history
- notifications list/read-all/read/delete
- rating submit/my-rating/report
- driver reviews and driver rating summary

### Expected
- these endpoints should return the correct owner-specific data
- admin-only support and SOS views should require admin auth

## 11. Admin Testing

Test these with `{{admin_token}}`:

- dashboard
- ride/user/driver/payment lists
- driver pending/verify/details/create
- withdrawal processing
- promo code create
- surge pricing create/remove
- ledger accounts/entries/audit-batch/balance
- SOS active/resolve
- support ticket admin actions
- config update
- bulk actions
- debug/password and reset-admin-password

### Expected
- admin-only actions must reject non-admin access
- mutation endpoints should be rate-limited
- bulk import should be safe to retry only if the same payload is intended

## 12. Bulk Admin Routes

These are the routes you asked about:

- `POST /api/v1/admin/bulk/verify-drivers`
- `POST /api/v1/admin/bulk/notify`
- `POST /api/v1/admin/bulk/import-drivers`
- `POST /api/v1/admin/bulk/update-driver-status`

### How to test them
- use `{{admin_token}}`
- send a small set of 2 to 3 IDs first
- verify partial success and error reporting
- then test a larger payload only after the small payload is stable

### Recommended expectations
- `bulk/verify-drivers`: verifies existing driver IDs only
- `bulk/notify`: queues messages, no duplicate sends in repeated accidental retries
- `bulk/import-drivers`: should create users and drivers when missing
- `bulk/update-driver-status`: should update driver state consistently

## 13. Suggested Location Test Data

Use these sample coordinates during testing:

- Bengaluru center: `12.9716, 77.5946`
- Indiranagar: `12.9784, 77.6408`
- Koramangala: `12.9352, 77.6245`
- Whitefield: `12.9698, 77.7500`

## 14. What to Check in Every Response

For each request, confirm:
- HTTP status code is expected
- `success` field is correct when present
- no internal database error leaks in the response
- IDs are returned and can be chained into the next step
- validation errors are specific and readable
- rate-limited endpoints fail safely when abused

## 15. Final Production Checklist

Before launch, confirm that:
- health endpoints are green
- auth flow works for rider, driver, and admin
- driver onboarding works
- ride request to completion works
- location updates and tracking work
- payment and refund flows work
- admin bulk actions work with small and large payloads
- bulk routes do not create duplicates on accidental retries

## 17. Quick Summary

If you want to know whether the backend is production ready in Postman, the critical path is:

1. `health`
2. `auth`
3. `driver register + online + location`
4. `ride estimate + request`
5. `driver accept + arrived + start + complete`
6. `track + ETA + fare`
7. `payment`
8. `admin` and `bulk admin`

If all of those are good, the backend is behaving like a production system.

## 16. Complete JSON Body Reference by Endpoint

The sections above show the main testing flow. This section is the one-to-one endpoint reference for every route that accepts JSON. GET and DELETE routes are intentionally omitted because they do not take a request body.

### Authentication

#### POST /api/v1/auth/otp/request

```json
{
  "phone": "9876543210"
}
```

#### POST /api/v1/auth/otp/verify

```json
{
  "phone": "9876543210",
  "email": "rider@example.com",
  "otp": "123456",
  "name": "Test Rider",
  "user_type": "rider"
}
```

#### POST /api/v1/auth/login

```json
{
  "identifier": "admin@rapido.com",
  "password": "admin123"
}
```

#### POST /api/v1/auth/refresh

```json
{
  "refresh_token": "{{refresh_token}}"
}
```

#### POST /api/v1/auth/google

```json
{
  "id_token": "google_id_token_here",
  "phone": "9876543210"
}
```

#### POST /api/v1/auth/logout

```json
{
  "refresh_token": "{{refresh_token}}"
}
```

#### PATCH /api/v1/auth/profile

```json
{
  "name": "Updated Rider",
  "email": "updated@example.com",
  "profile_image": "https://example.com/profile.png"
}
```

#### POST /api/v1/auth/password/set

```json
{
  "password": "yourpassword123"
}
```

#### POST /api/v1/auth/password/change

```json
{
  "old_password": "yourpassword123",
  "new_password": "newpassword456"
}
```

#### POST /api/v1/auth/emergency-contacts

```json
{
  "name": "Mom",
  "phone": "9999999999",
  "relationship": "parent",
  "priority": 1
}
```

#### PUT /api/v1/auth/emergency-contacts/{{contact_id}}

```json
{
  "name": "Dad",
  "phone": "9888888888",
  "relationship": "parent",
  "priority": 2
}
```

#### POST /api/v1/sos/trigger

```json
{
  "latitude": 12.9716,
  "longitude": 77.5946,
  "address": "MG Road, Bengaluru",
  "ride_id": "{{ride_id}}"
}
```

### Driver Onboarding and Live Location

#### POST /api/v1/drivers/register

```json
{
  "license_number": "KA01DL1234",
  "license_image": "https://example.com/license.jpg",
  "license_expiry": "2027-12-31T00:00:00Z",
  "rc_number": "KA01RC1234",
  "rc_image": "https://example.com/rc.jpg",
  "aadhaar_number": "123412341234",
  "aadhaar_image": "https://example.com/aadhaar.jpg",
  "vehicle_type": "car",
  "vehicle_make": "Toyota",
  "vehicle_model": "Etios",
  "vehicle_year": 2022,
  "vehicle_color": "White",
  "vehicle_number_plate": "KA01AB1234",
  "fuel_type": "petrol",
  "vehicle_image": "https://example.com/vehicle.jpg",
  "languages": ["English", "Kannada"]
}
```

#### PATCH /api/v1/drivers/profile

```json
{
  "languages": ["English", "Kannada", "Hindi"]
}
```

#### POST /api/v1/drivers/online

```json
{
  "lat": 12.9716,
  "lng": 77.5946
}
```

#### POST /api/v1/drivers/location

```json
{
  "lat": 12.972,
  "lng": 77.5955,
  "accuracy": 12.5
}
```

### Ride Creation and Lifecycle

#### POST /api/v1/rides

```json
{
  "vehicle_type": "car",
  "pickup_lat": 12.9716,
  "pickup_lng": 77.5946,
  "pickup_address": "MG Road, Bengaluru",
  "dropoff_lat": 12.9784,
  "dropoff_lng": 77.6408,
  "dropoff_address": "Indiranagar, Bengaluru",
  "promo_code": "",
  "payment_method": "cash",
  "preferences": {
    "female_driver_only": false,
    "ac_required": true,
    "silence_mode": false,
    "music": false,
    "luggage_space": true
  }
}
```

#### POST /api/v1/rides/{{ride_id}}/cancel

```json
{
  "reason": "change_of_plans"
}
```

#### POST /api/v1/rides/{{ride_id}}/retry

```json
{}
```

#### POST /api/v1/rides/{{ride_id}}/apply-promo

```json
{
  "promo_code": "RAPIDO50"
}
```

#### POST /api/v1/rides/{{ride_id}}/accept

```json
{}
```

#### POST /api/v1/rides/{{ride_id}}/reject

```json
{}
```

#### POST /api/v1/rides/{{ride_id}}/arrived

```json
{}
```

#### POST /api/v1/rides/{{ride_id}}/start

```json
{
  "otp": "{{ride_otp}}"
}
```

#### POST /api/v1/rides/{{ride_id}}/complete

```json
{
  "final_lat": 12.9784,
  "final_lng": 77.6408
}
```

#### PATCH /api/v1/rides/{{ride_id}}/status

```json
{
  "status": "started",
  "otp": "{{ride_otp}}",
  "final_lat": 12.9721,
  "final_lng": 77.5959
}
```

#### POST /api/v1/rides/{{ride_id}}/reassign

```json
{
  "reason": "driver_unavailable",
  "preferred_driver_types": ["car", "suv"],
  "priority": "high"
}
```

### Scheduled Rides

#### POST /api/v1/rides/schedule

```json
{
  "vehicle_type": "car",
  "pickup_lat": 12.9716,
  "pickup_lng": 77.5946,
  "pickup_address": "MG Road, Bengaluru",
  "dropoff_lat": 12.9784,
  "dropoff_lng": 77.6408,
  "dropoff_address": "Indiranagar, Bengaluru",
  "scheduled_at": "2026-05-14T15:30:00Z",
  "notes": "Airport drop for tomorrow morning",
  "preferences": {
    "female_driver_only": false,
    "ac_required": true,
    "luggage_space": true
  }
}
```

#### PUT /api/v1/rides/scheduled/{{scheduled_ride_id}}

```json
{
  "vehicle_type": "car",
  "pickup_lat": 12.9716,
  "pickup_lng": 77.5946,
  "pickup_address": "MG Road, Bengaluru",
  "dropoff_lat": 12.9784,
  "dropoff_lng": 77.6408,
  "dropoff_address": "Indiranagar, Bengaluru",
  "scheduled_at": "2026-05-14T16:00:00Z",
  "notes": "Updated schedule",
  "preferences": {
    "female_driver_only": false,
    "ac_required": true,
    "luggage_space": true
  }
}
```

#### POST /api/v1/rides/scheduled/{{scheduled_ride_id}}/cancel

```json
{}
```

### Payments and Wallet

#### POST /api/v1/wallet/add-money

```json
{
  "amount": 500,
  "method": "upi"
}
```

#### POST /api/v1/withdrawals

```json
{
  "amount": 250,
  "method": "bank_transfer",
  "bank_details": {
    "account_name": "Test Rider",
    "account_number": "1234567890",
    "ifsc": "HDFC0001234",
    "bank_name": "HDFC Bank"
  }
}
```

#### POST /api/v1/payments/rides/{{ride_id}}/pay

```json
{
  "method": "upi"
}
```

#### POST /api/v1/payments/rides/{{ride_id}}/retry

```json
{
  "method": "upi"
}
```

#### POST /api/v1/payments/{{payment_id}}/refund

```json
{
  "amount": 100,
  "reason": "duplicate_payment"
}
```

#### POST /api/v1/payments/methods/card

```json
{
  "card_number": "4111111111111111",
  "expiry_month": 12,
  "expiry_year": 2028,
  "cvv": "123",
  "cardholder_name": "Test Rider",
  "card_type": "debit",
  "nickname": "Primary card",
  "set_as_default": true,
  "billing_address": "123 MG Road, Bengaluru"
}
```

#### POST /api/v1/payments/methods/upi

```json
{
  "vpa": "testuser@upi",
  "nickname": "Primary UPI",
  "set_as_default": true
}
```

### Support, Ratings, and Reports

#### POST /api/v1/users/support/tickets

```json
{
  "category": "payment",
  "priority": "high",
  "subject": "Refund pending",
  "description": "Payment was deducted twice",
  "ride_id": "{{ride_id}}"
}
```

#### POST /api/v1/users/support/tickets/{{ticket_id}}/messages

```json
{
  "message": "Please check the refund status"
}
```

#### PUT /api/v1/admin/support/tickets/{{ticket_id}}

```json
{
  "status": "in_progress",
  "priority": "high",
  "assigned_to": "admin-uuid-here",
  "resolution": "Refund initiated",
  "refund_amount": 100
}
```

#### POST /api/v1/admin/support/tickets/{{ticket_id}}/messages

```json
{
  "message": "We are checking this now",
  "is_internal": true
}
```

#### POST /api/v1/rides/{{ride_id}}/rate

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

#### POST /api/v1/ratings/{{rating_id}}/report

```json
{
  "reason": "abusive_language",
  "details": "Rating contains inappropriate content"
}
```

#### POST /api/v1/admin/ratings/reports/{{rating_id}}/resolve

```json
{
  "action": "remove",
  "notes": "Abusive language"
}
```

### Admin Operations

#### POST /api/v1/admin/withdrawals/process

```json
{
  "withdrawal_id": "{{withdrawal_id}}",
  "approved": true,
  "rejection_reason": ""
}
```

#### POST /api/v1/admin/surge-pricing

```json
{
  "area_name": "Bengaluru East",
  "lat": 12.9716,
  "lng": 77.5946,
  "radius_km": 5,
  "multiplier": 1.5,
  "reason": "Peak demand",
  "duration_hours": 2
}
```

#### POST /api/v1/admin/promo-codes

```json
{
  "code": "WELCOME50",
  "description": "50 percent off for new users",
  "discount_type": "percentage",
  "discount_value": 50,
  "max_discount": 100,
  "min_ride_amount": 200,
  "max_uses": 500,
  "max_uses_per_user": 1,
  "vehicle_types": ["car", "bike"],
  "start_date": "2026-05-14T00:00:00Z",
  "end_date": "2026-06-14T00:00:00Z"
}
```

#### POST /api/v1/admin/ledger/audit-batch

```json
{
  "batch_id": "{{batch_id}}"
}
```

#### PATCH /api/v1/admin/config

```json
{
  "key": "app_name",
  "value": "Rapido Production"
}
```

#### POST /api/v1/admin/drivers/create

```json
{
  "name": "New Driver",
  "email": "newdriver@example.com",
  "phone": "9876543222",
  "password": "driverpass123",
  "license_number": "KA01DL5678",
  "license_image": "https://example.com/license.jpg",
  "license_expiry": "2027-12-31T00:00:00Z",
  "rc_number": "KA01RC5678",
  "rc_image": "https://example.com/rc.jpg",
  "aadhaar_number": "567856785678",
  "aadhaar_image": "https://example.com/aadhaar.jpg",
  "languages": ["English", "Kannada"],
  "vehicle_type": "car",
  "vehicle_make": "Honda",
  "vehicle_model": "City",
  "vehicle_year": 2021,
  "vehicle_color": "Black",
  "vehicle_number_plate": "KA01CD5678",
  "fuel_type": "petrol",
  "vehicle_image": "https://example.com/vehicle.jpg",
  "auto_verify": false
}
```

#### POST /api/v1/admin/drivers/{{driver_id}}/verify

```json
{
  "verified": true,
  "notes": "Documents checked and approved"
}
```

Alternative legacy form:

```json
{
  "action": "approve",
  "notes": "Documents checked and approved"
}
```

#### POST /api/v1/admin/sos/{{sos_id}}/resolve

```json
{
  "notes": "Resolved and user contacted"
}
```

### Bulk Admin Operations

#### POST /api/v1/admin/bulk/verify-drivers

```json
{
  "driver_ids": [
    "11111111-1111-1111-1111-111111111111",
    "22222222-2222-2222-2222-222222222222"
  ],
  "notes": "Batch verification from onboarding queue"
}
```

#### POST /api/v1/admin/bulk/notify

```json
{
  "user_ids": [
    "33333333-3333-3333-3333-333333333333",
    "44444444-4444-4444-4444-444444444444"
  ],
  "user_type": "driver",
  "title": "Maintenance Window",
  "body": "The app will be briefly unavailable for maintenance.",
  "channels": ["push", "sms"]
}
```

#### POST /api/v1/admin/bulk/import-drivers

```json
{
  "drivers": [
    {
      "name": "Driver One",
      "phone": "9876543301",
      "email": "driver1@example.com",
      "city": "Bengaluru",
      "vehicle_type": "car",
      "vehicle_number": "KA01AA0001"
    },
    {
      "name": "Driver Two",
      "phone": "9876543302",
      "email": "driver2@example.com",
      "city": "Bengaluru",
      "vehicle_type": "bike",
      "vehicle_number": "KA01BB0002"
    }
  ]
}
```

#### POST /api/v1/admin/bulk/update-driver-status

```json
{
  "driver_ids": [
    "11111111-1111-1111-1111-111111111111",
    "22222222-2222-2222-2222-222222222222"
  ],
  "status": "inactive",
  "reason": "Compliance review"
}
```
