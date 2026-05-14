# Rapido Backend - Complete Models and Tables Documentation

## Overview
This document provides a detailed breakdown of all database models and tables in the Rapido ride-sharing application backend. The system uses PostgreSQL with Go/GORM for ORM.

---

## 1. USER MANAGEMENT MODELS

### 1.1 PublicUser (public_users table)
**Purpose**: Core user account for both riders and drivers
**Table Name**: `public_users`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Unique user identifier |
| name | VARCHAR | | User's full name |
| email | VARCHAR | | Email address |
| phone | VARCHAR | | Phone number |
| password_hash | VARCHAR | INDEXED | Hashed password (not exposed in JSON) |
| provider | VARCHAR | | OAuth provider (google, local, etc.) |
| provider_id | VARCHAR | | Third-party provider ID |
| email_verified | BOOLEAN | | Email verification status |
| profile_image | VARCHAR | | Avatar/profile picture URL |
| role | VARCHAR | | User role (rider, driver, admin) |
| google_id | VARCHAR | INDEXED | Google OAuth ID |
| is_active | BOOLEAN | | Account active status |
| latitude | FLOAT | | Last known latitude |
| longitude | FLOAT | | Last known longitude |
| address | VARCHAR | | Last known address |
| location_updated_at | TIMESTAMP | | When location was last updated |
| created_at | TIMESTAMP | | Account creation time |
| updated_at | TIMESTAMP | | Last update time |

**Alias**: User (legacy alias for backward compatibility)

---

### 1.2 OTP (otps table)
**Purpose**: One-Time Passwords for authentication
**Table Name**: `otps`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | OTP record ID |
| phone | VARCHAR | INDEXED | Phone number the OTP was sent to |
| code | VARCHAR | | 6-digit verification code |
| purpose | VARCHAR | | Purpose of OTP: login, ride, withdrawal |
| expires_at | TIMESTAMP | | When OTP expires |
| used_at | TIMESTAMP | NULLABLE | When OTP was used |
| created_at | TIMESTAMP | | Creation time |

**Status Values**: login, ride, withdrawal

---

### 1.3 RefreshToken (refresh_tokens table)
**Purpose**: Token refresh mechanism for session management
**Table Name**: `refresh_tokens`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Token ID |
| user_id | UUID | INDEXED | User who owns this token |
| token | VARCHAR | UNIQUE INDEX | Token value |
| expires_at | TIMESTAMP | | Token expiration time |
| revoked_at | TIMESTAMP | NULLABLE | When token was revoked |
| created_at | TIMESTAMP | | Creation time |

---

### 1.4 Device (devices table)
**Purpose**: Track registered user devices for security
**Table Name**: `devices`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Device ID |
| user_id | UUID | INDEXED | Device owner |
| user_type | VARCHAR | | rider or driver |
| device_id | VARCHAR | INDEXED | Unique device identifier |
| device_name | VARCHAR | | Human-readable device name |
| device_model | VARCHAR | | Phone model (e.g., iPhone 14) |
| os_version | VARCHAR | | OS version number |
| app_version | VARCHAR | | App version |
| ip_address | VARCHAR | | Device IP address |
| is_trusted | BOOLEAN | | Is device trusted/verified |
| status | VARCHAR | | active, revoked, inactive |
| last_active_at | TIMESTAMP | | Last activity timestamp |
| created_at | TIMESTAMP | | Registration time |
| updated_at | TIMESTAMP | | Last update |
| deleted_at | TIMESTAMP | | Soft delete timestamp |

**Status Values**: active, revoked, inactive

---

### 1.5 DeviceSession (device_sessions table)
**Purpose**: Active sessions on user devices
**Table Name**: `device_sessions`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Session ID |
| device_id | UUID | INDEXED | Associated device |
| user_id | UUID | INDEXED | User in session |
| token_hash | VARCHAR | | Hashed session token |
| is_active | BOOLEAN | | Session active status |
| last_activity | TIMESTAMP | | Last user activity |
| created_at | TIMESTAMP | | Session start time |
| expires_at | TIMESTAMP | | Session expiration |
| deleted_at | TIMESTAMP | | Soft delete |

---

## 2. DRIVER MANAGEMENT MODELS

### 2.1 Driver (drivers table)
**Purpose**: Driver profile and verification details
**Table Name**: `drivers`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Driver ID |
| user_id | UUID | UNIQUE INDEX | Reference to PublicUser |
| license_number | VARCHAR | UNIQUE INDEX | Driver's license number |
| license_image | VARCHAR | | License image URL |
| license_expiry | TIMESTAMP | | License expiration date |
| rc_number | VARCHAR | UNIQUE INDEX | Vehicle Registration Certificate |
| rc_image | VARCHAR | | RC image URL |
| aadhaar_number | VARCHAR | UNIQUE INDEX | Aadhaar ID (masked in JSON) |
| aadhaar_image | VARCHAR | | Aadhaar image URL |
| is_verified | BOOLEAN | | Verification status |
| is_online | BOOLEAN | | Currently online status |
| is_active | BOOLEAN | | Account active |
| rating | FLOAT | | Average rating (default 5.0) |
| total_rides | INT | | Total completed rides |
| acceptance_score | FLOAT | | Acceptance rate % |
| cancellation_rate | FLOAT | | Cancellation rate % |
| preferred_locations | TEXT[] | | Preferred pickup areas |
| languages | TEXT[] | | Languages driver speaks |
| is_female | BOOLEAN | | Female/male indicator for preferences |
| verified_by | UUID | NULLABLE | Admin who verified |
| verified_at | TIMESTAMP | | Verification timestamp |
| created_at | TIMESTAMP | | Driver registration |
| updated_at | TIMESTAMP | | Last update |
| deleted_at | TIMESTAMP | | Soft delete |

---

### 2.2 DriverLocation (driver_locations table)
**Purpose**: Real-time driver location tracking
**Table Name**: `driver_locations`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Location record ID |
| driver_id | UUID | UNIQUE INDEX | Driver being tracked |
| latitude | FLOAT | | Current latitude |
| longitude | FLOAT | | Current longitude |
| accuracy | FLOAT | | GPS accuracy in meters |
| heading | FLOAT | | Direction heading (0-360°) |
| speed | FLOAT | | Current speed in km/h |
| battery_level | INT | | Device battery percentage |
| updated_at | TIMESTAMP | | Last update time |

---

### 2.3 Vehicle (vehicles table)
**Purpose**: Driver vehicle details
**Table Name**: `vehicles`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Vehicle ID |
| driver_id | UUID | INDEXED | Vehicle owner |
| type | VARCHAR | | bike, auto, car |
| make | VARCHAR | | Vehicle brand (Honda, Maruti, etc.) |
| model | VARCHAR | | Vehicle model |
| year | INT | | Manufacturing year |
| color | VARCHAR | | Vehicle color |
| number_plate | VARCHAR | UNIQUE INDEX | License plate |
| fuel_type | VARCHAR | | petrol, diesel, electric, cng |
| is_active | BOOLEAN | | Vehicle currently in use |
| vehicle_image | VARCHAR | | Photo of vehicle |
| created_at | TIMESTAMP | | Added date |
| updated_at | TIMESTAMP | | Last update |

**Vehicle Types**: bike, auto, car

---

### 2.4 DriverDocument (driver_documents table)
**Purpose**: Additional verification documents
**Table Name**: `driver_documents`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Document ID |
| driver_id | UUID | INDEXED | Driver |
| type | VARCHAR | | license, rc, aadhaar, insurance, permit |
| number | VARCHAR | | Document number |
| image_url | VARCHAR | | Document image |
| status | VARCHAR | | pending, approved, rejected |
| rejection_reason | VARCHAR | | Why rejected |
| verified_by | UUID | | Verifying admin |
| verified_at | TIMESTAMP | | Verification time |
| expires_at | TIMESTAMP | | Document expiry |
| created_at | TIMESTAMP | | Upload time |
| updated_at | TIMESTAMP | | Last update |

**Status Values**: pending, approved, rejected

---

### 2.5 DriverEarnings (driver_earnings table)
**Purpose**: Earnings summary for drivers
**Table Name**: `driver_earnings`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Record ID |
| driver_id | UUID | INDEXED | Driver |
| total_earnings | FLOAT | | Lifetime earnings |
| total_rides | INT | | Lifetime completed rides |
| pending_amount | FLOAT | | Unpaid earnings |
| withdrawn_amount | FLOAT | | Total withdrawn |
| current_balance | FLOAT | | Available balance |
| daily_earnings | FLOAT | | Today's earnings |
| weekly_earnings | FLOAT | | This week's earnings |
| monthly_earnings | FLOAT | | This month's earnings |
| last_updated | TIMESTAMP | | Last update |
| created_at | TIMESTAMP | | Record created |

---

### 2.6 DriverStatusLog (driver_status_logs table)
**Purpose**: Audit trail of driver status changes
**Table Name**: `driver_status_logs`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Log ID |
| driver_id | UUID | INDEXED | Driver |
| from_status | VARCHAR | | Previous status |
| to_status | VARCHAR | | New status |
| reason | VARCHAR | | Change reason |
| metadata | JSONB | | Additional context |
| created_at | TIMESTAMP | | Change time |

---

## 3. RIDE MANAGEMENT MODELS

### 3.1 Ride (rides table)
**Purpose**: Core ride record
**Table Name**: `rides`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Ride ID |
| rider_id | UUID | INDEXED | Requesting user |
| driver_id | UUID | NULLABLE INDEXED | Assigned driver |
| vehicle_id | UUID | NULLABLE | Vehicle used |
| status | VARCHAR | INDEXED | Current ride status |
| vehicle_type | VARCHAR | | bike, auto, car_go, car_x |
| pickup_address | VARCHAR | | Pickup location name |
| pickup_latitude | FLOAT | | Pickup latitude |
| pickup_longitude | FLOAT | | Pickup longitude |
| pickup_updated_at | TIMESTAMP | | Pickup location update |
| dropoff_address | VARCHAR | | Dropoff location name |
| dropoff_latitude | FLOAT | | Dropoff latitude |
| dropoff_longitude | FLOAT | | Dropoff longitude |
| dropoff_updated_at | TIMESTAMP | | Dropoff location update |
| estimated_distance | FLOAT | | Distance in km |
| estimated_duration | INT | | Duration in minutes |
| estimated_fare | FLOAT | | Estimated cost |
| actual_distance | FLOAT | | Actual distance traveled |
| actual_duration | INT | | Actual duration |
| final_fare | FLOAT | | Final amount charged |
| base_fare | FLOAT | | Minimum charge |
| per_km_rate | FLOAT | | Rate per kilometer |
| per_min_rate | FLOAT | | Rate per minute |
| surge_multiplier | FLOAT | | Dynamic pricing multiplier |
| surge_amount | FLOAT | | Extra charge from surge |
| platform_fee | FLOAT | | App fee |
| tax_amount | FLOAT | | GST/tax |
| promo_code | VARCHAR | | Discount code used |
| discount_amount | FLOAT | | Discount value |
| payment_method | VARCHAR | | cash, upi, card, wallet |
| payment_status | VARCHAR | | pending, completed, failed, refunded |
| ride_otp | VARCHAR | | Rider verification code |
| idempotency_key | VARCHAR | UNIQUE INDEX | Idempotent request ID |
| pref_ac_required | BOOLEAN | | AC preference |
| pref_female_driver_only | BOOLEAN | | Female driver only |
| pref_luggage_space | BOOLEAN | | Luggage space needed |
| pref_silence_mode | BOOLEAN | | Silence mode preference |
| pref_music | BOOLEAN | | Music preference |
| cancellation_reason | VARCHAR | | Reason if cancelled |
| cancelled_by | UUID | | Who cancelled |
| cancellation_time | TIMESTAMP | | When cancelled |
| cancellation_fee | FLOAT | | Charge for cancellation |
| requested_at | TIMESTAMP | | Request time |
| accepted_at | TIMESTAMP | | Acceptance time |
| arrived_at | TIMESTAMP | | Driver arrival time |
| started_at | TIMESTAMP | | Ride start time |
| completed_at | TIMESTAMP | | Ride completion time |
| created_at | TIMESTAMP | | Record created |
| updated_at | TIMESTAMP | | Last update |
| deleted_at | TIMESTAMP | | Soft delete |

**Status Values**: requested, driver_assigned, driver_arrived, ongoing, completed, cancelled, no_driver_found

**Vehicle Types**: bike, auto, car_go, car_x

**Cancellation Reasons**: rider_cancelled, driver_cancelled, no_driver_available, driver_not_found, other

---

### 3.2 RideLocation (ride_locations table)
**Purpose**: Historical location tracking during ride
**Table Name**: `ride_locations`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Location ID |
| ride_id | UUID | INDEXED | Ride being tracked |
| latitude | FLOAT | | Position latitude |
| longitude | FLOAT | | Position longitude |
| accuracy | FLOAT | | GPS accuracy |
| speed | FLOAT | | Speed at this point |
| heading | FLOAT | | Direction heading |
| altitude | FLOAT | | Altitude/elevation |
| created_at | TIMESTAMP | | Recording time |

---

### 3.3 RideRequestLog (ride_request_logs table)
**Purpose**: Request sent to drivers for matching
**Table Name**: `ride_request_logs`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Log ID |
| ride_id | UUID | INDEXED | Ride |
| driver_id | UUID | INDEXED | Driver requested |
| status | VARCHAR | | pending, accepted, rejected, timeout |
| rejected_at | TIMESTAMP | | Rejection time |
| created_at | TIMESTAMP | | Request sent time |

---

### 3.4 RideMatch (ride_matches table)
**Purpose**: Driver matching and scoring
**Table Name**: `ride_matches`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Match ID |
| ride_id | UUID | INDEXED | Ride |
| driver_id | UUID | INDEXED | Candidate driver |
| distance | FLOAT | | Distance in km from pickup |
| eta | INT | | Minutes to pickup |
| driver_rating | FLOAT | | Driver's current rating |
| acceptance_score | FLOAT | | Driver's acceptance rate |
| match_score | FLOAT | | Calculated match quality score |
| notified_at | TIMESTAMP | | When driver was notified |
| responded_at | TIMESTAMP | | Driver response time |
| response | VARCHAR | | accepted, rejected, timeout |
| created_at | TIMESTAMP | | Match created |

---

### 3.5 RideStatusLog (ride_status_logs table)
**Purpose**: Audit trail of ride status changes
**Table Name**: `ride_status_logs`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Log ID |
| ride_id | UUID | INDEXED | Ride |
| from_status | VARCHAR | | Previous status |
| to_status | VARCHAR | | New status |
| reason | VARCHAR | | Change reason |
| actor_id | UUID | NULLABLE | Who made change |
| actor_type | VARCHAR | | rider, driver, system, admin |
| location_lat | FLOAT | | Location at change |
| location_lng | FLOAT | | Location at change |
| metadata | JSONB | | Additional context |
| created_at | TIMESTAMP | | Change time |

---

## 4. PAYMENT & FINANCIAL MODELS

### 4.1 Payment (payments table)
**Purpose**: Payment records for rides
**Table Name**: `payments`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Payment ID |
| ride_id | UUID | INDEXED | Associated ride |
| payer_id | UUID | INDEXED | Who paid |
| payee_id | UUID | INDEXED | Who received |
| amount | FLOAT | | Payment amount |
| currency | VARCHAR | | INR (default) |
| method | VARCHAR | | Payment method |
| status | VARCHAR | | pending, completed, failed, refunded |
| transaction_id | VARCHAR | UNIQUE INDEX | Payment gateway transaction ID |
| gateway | VARCHAR | | razorpay, stripe, etc. |
| gateway_ref | VARCHAR | | Gateway reference |
| idempotency_key | VARCHAR | UNIQUE INDEX | Idempotent key |
| failure_reason | VARCHAR | | Why payment failed |
| refunded_at | TIMESTAMP | | Refund date |
| refund_amount | FLOAT | | Amount refunded |
| metadata | JSONB | | Additional data |
| created_at | TIMESTAMP | | Payment date |
| updated_at | TIMESTAMP | | Last update |
| deleted_at | TIMESTAMP | | Soft delete |

**Status Values**: pending, completed, failed, refunded, disputed

---

### 4.2 PaymentMethod (payment_methods table)
**Purpose**: Saved payment methods
**Table Name**: `payment_methods`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Method ID |
| user_id | UUID | INDEXED | User |
| type | VARCHAR | | card, upi, wallet, cash |
| is_default | BOOLEAN | | Default payment method |
| status | VARCHAR | | active, expired, disabled |
| nickname | VARCHAR | | User's label |
| created_at | TIMESTAMP | | Added date |
| updated_at | TIMESTAMP | | Last update |
| deleted_at | TIMESTAMP | | Soft delete |

**Sub-objects for Card Details** (encrypted):
- card_number (encrypted)
- last4 (last 4 digits, unencrypted)
- card_type (credit/debit)
- card_brand (visa, mastercard, amex, rupay, discover)
- expiry_month, expiry_year
- cardholder_name
- token (payment gateway token)
- billing_address
- is_auto_debit

**Sub-objects for UPI Details** (encrypted):
- vpa (UPI ID)
- bank_name
- bank_account (encrypted)
- account_type (savings/current)
- ifsc_code (encrypted)
- is_auto_debit

---

### 4.3 Wallet (wallets table)
**Purpose**: User wallet for prepaid balance
**Table Name**: `wallets`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Wallet ID |
| user_id | UUID | UNIQUE INDEX | Wallet owner |
| balance | FLOAT | | Current balance |
| currency | VARCHAR | | INR (default) |
| is_active | BOOLEAN | | Active status |
| daily_limit | FLOAT | | Daily limit |
| monthly_limit | FLOAT | | Monthly limit |
| auto_recharge | BOOLEAN | | Auto top-up enabled |
| auto_recharge_amount | FLOAT | | Auto top-up amount |
| created_at | TIMESTAMP | | Created |
| updated_at | TIMESTAMP | | Updated |
| deleted_at | TIMESTAMP | | Soft delete |

---

### 4.4 Transaction (transactions table)
**Purpose**: Financial transaction record
**Table Name**: `transactions`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Transaction ID |
| user_id | UUID | INDEXED | User involved |
| type | VARCHAR | | ride_payment, wallet_topup, withdrawal, driver_payout, refund, penalty, commission |
| amount | FLOAT | | Amount |
| currency | VARCHAR | | INR (default) |
| status | VARCHAR | | pending, completed, failed |
| description | VARCHAR | | Transaction description |
| reference_id | VARCHAR | | Related resource (ride_id, withdrawal_id, etc.) |
| payment_id | UUID | NULLABLE | Associated payment |
| wallet_balance | FLOAT | | Balance after transaction |
| metadata | JSONB | | Additional data |
| created_at | TIMESTAMP | | Transaction date |
| updated_at | TIMESTAMP | | Last update |

**Types**: ride_payment, wallet_topup, wallet_withdrawal, driver_payout, refund, penalty, commission

---

### 4.5 Commission (commissions table)
**Purpose**: Platform commission on rides
**Table Name**: `commissions`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Commission ID |
| ride_id | UUID | UNIQUE INDEX | Related ride |
| driver_id | UUID | INDEXED | Driver |
| total_fare | FLOAT | | Total fare amount |
| platform_commission | FLOAT | | Platform cut |
| driver_earnings | FLOAT | | Driver's share |
| tax_amount | FLOAT | | Tax on commission |
| service_fee | FLOAT | | Additional service fee |
| platform_percent | FLOAT | | Commission percentage |
| paid_at | TIMESTAMP | | Payment date |
| settlement_id | VARCHAR | | Settlement batch ID |
| created_at | TIMESTAMP | | Created |
| updated_at | TIMESTAMP | | Updated |

---

### 4.6 Withdrawal (withdrawals table)
**Purpose**: Driver withdrawal requests
**Table Name**: `withdrawals`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Withdrawal ID |
| driver_id | UUID | INDEXED | Driver requesting |
| amount | FLOAT | | Withdrawal amount |
| currency | VARCHAR | | INR (default) |
| status | VARCHAR | | pending, processing, completed, rejected |
| method | VARCHAR | | bank_transfer, upi |
| bank_details | JSONB | | Bank account details |
| processed_at | TIMESTAMP | | Processing date |
| processed_by | UUID | | Admin who processed |
| rejection_reason | VARCHAR | | Reason if rejected |
| transaction_id | UUID | | Related transaction |
| requested_at | TIMESTAMP | | Request date |
| created_at | TIMESTAMP | | Created |
| updated_at | TIMESTAMP | | Updated |

**Status Values**: pending, processing, completed, rejected

---

### 4.7 Invoice (invoices table)
**Purpose**: GST invoices for rides
**Table Name**: `invoices`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Invoice ID |
| ride_id | UUID | UNIQUE INDEX | Associated ride |
| payment_id | UUID | | Payment record |
| invoice_number | VARCHAR | UNIQUE INDEX | Invoice number |
| customer_name | VARCHAR | | Rider name |
| customer_gst | VARCHAR | | Customer GST ID |
| amount | FLOAT | | Pre-tax amount |
| tax_amount | FLOAT | | Tax amount |
| total_amount | FLOAT | | Total including tax |
| gst_percent | FLOAT | | GST rate (default 18%) |
| invoice_url | VARCHAR | | PDF URL |
| generated_at | TIMESTAMP | | Generation time |
| sent_at | TIMESTAMP | | Email sent time |
| created_at | TIMESTAMP | | Created |

---

### 4.8 LedgerAccount (ledger_accounts table)
**Purpose**: Core accounting ledger
**Table Name**: `ledger_accounts`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Account ID |
| account_key | VARCHAR | UNIQUE INDEX | Account identifier |
| account_type | VARCHAR | | user_wallet, driver_wallet, driver_earnings, platform_revenue, payment_clearing, topup_clearing, withdrawal_clearing, refund_clearing |
| owner_id | UUID | NULLABLE | Account owner |
| currency | VARCHAR | | INR (default) |
| balance | FLOAT | | Current balance |
| is_active | BOOLEAN | | Active status |
| created_at | TIMESTAMP | | Created |
| updated_at | TIMESTAMP | | Updated |
| deleted_at | TIMESTAMP | | Soft delete |

---

### 4.9 LedgerEntry (ledger_entries table)
**Purpose**: Individual ledger transactions
**Table Name**: `ledger_entries`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Entry ID |
| batch_id | UUID | INDEXED | Transaction batch |
| account_id | UUID | INDEXED | Account |
| direction | VARCHAR | | debit or credit |
| amount | FLOAT | | Amount |
| balance_before | FLOAT | | Balance before |
| balance_after | FLOAT | | Balance after |
| currency | VARCHAR | | INR (default) |
| reference_type | VARCHAR | | ride, payment, withdrawal, etc. |
| reference_id | VARCHAR | INDEXED | Related resource ID |
| description | VARCHAR | | Entry description |
| created_at | TIMESTAMP | | Created |

---

## 5. RATING & FEEDBACK MODELS

### 5.1 Rating (ratings table)
**Purpose**: Ride ratings and reviews
**Table Name**: `ratings`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Rating ID |
| ride_id | UUID | UNIQUE INDEX | Rated ride |
| rider_id | UUID | INDEXED | Rating author |
| driver_id | UUID | INDEXED | Driver rated |
| rider_rating | INT | | Rider rating (1-5) |
| driver_rating | INT | | Driver rating (1-5, required) |
| rider_review | VARCHAR | | Rider's review text |
| driver_review | VARCHAR | | Driver's review text |
| rider_rated_at | TIMESTAMP | | When rider rated |
| driver_rated_at | TIMESTAMP | | When driver rated |
| cat_cleanliness | INT | | Cleanliness rating (1-5) |
| cat_punctuality | INT | | Punctuality rating (1-5) |
| cat_driving_skill | INT | | Driving skill rating (1-5) |
| cat_behavior | INT | | Behavior rating (1-5) |
| cat_route_knowledge | INT | | Route knowledge rating (1-5) |
| is_reported | BOOLEAN | | Flagged for review |
| report_reason | VARCHAR | | Reason if reported |
| report_details | VARCHAR | | Report details |
| created_at | TIMESTAMP | | Created |
| updated_at | TIMESTAMP | | Updated |
| deleted_at | TIMESTAMP | | Soft delete |

---

### 5.2 DriverRatingSummary (driver_rating_summaries table)
**Purpose**: Cached rating statistics for drivers
**Table Name**: `driver_rating_summaries`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Summary ID |
| driver_id | UUID | UNIQUE INDEX | Driver |
| average_rating | FLOAT | | Average rating (default 5.0) |
| total_ratings | INT | | Total ratings received |
| five_star_count | INT | | Number of 5-star ratings |
| four_star_count | INT | | Number of 4-star ratings |
| three_star_count | INT | | Number of 3-star ratings |
| two_star_count | INT | | Number of 2-star ratings |
| one_star_count | INT | | Number of 1-star ratings |
| cleanliness_avg | FLOAT | | Average cleanliness score |
| punctuality_avg | FLOAT | | Average punctuality score |
| driving_skill_avg | FLOAT | | Average driving skill score |
| behavior_avg | FLOAT | | Average behavior score |
| last_updated | TIMESTAMP | | Last update |
| created_at | TIMESTAMP | | Created |

---

## 6. NOTIFICATION MODELS

### 6.1 Notification (notifications table)
**Purpose**: System notifications to users
**Table Name**: `notifications`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Notification ID |
| user_id | UUID | INDEXED | Recipient |
| type | VARCHAR | | ride_request, ride_accepted, driver_arrived, ride_started, ride_completed, payment_received, promo_code, driver_verified, sos_alert, system, marketing |
| title | VARCHAR | | Notification title |
| body | VARCHAR | | Notification content |
| data | JSONB | | Additional data |
| channels | TEXT[] | | push, sms, email, in_app |
| status | VARCHAR | | pending, sent, delivered, read, failed |
| priority | VARCHAR | | low, normal, high, urgent |
| sent_at | TIMESTAMP | | Sent time |
| read_at | TIMESTAMP | | Read time |
| error | VARCHAR | | Error message if failed |
| created_at | TIMESTAMP | | Created |
| updated_at | TIMESTAMP | | Updated |
| deleted_at | TIMESTAMP | | Soft delete |

**Types**: ride_request, ride_accepted, driver_arrived, ride_started, ride_completed, payment_received, promo_code, driver_verified, sos_alert, system, marketing

**Channels**: push, sms, email, in_app

---

### 6.2 NotificationPreference (notification_preferences table)
**Purpose**: User notification settings
**Table Name**: `notification_preferences`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Preference ID |
| user_id | UUID | UNIQUE INDEX | User |
| push_enabled | BOOLEAN | | Push notifications enabled |
| sms_enabled | BOOLEAN | | SMS enabled |
| email_enabled | BOOLEAN | | Email enabled |
| ride_updates_push | BOOLEAN | | Ride updates via push |
| ride_updates_sms | BOOLEAN | | Ride updates via SMS |
| promotions_push | BOOLEAN | | Promotions via push |
| promotions_email | BOOLEAN | | Promotions via email |
| marketing_emails | BOOLEAN | | Marketing emails |
| safety_alerts_push | BOOLEAN | | Safety alerts via push |
| safety_alerts_sms | BOOLEAN | | Safety alerts via SMS |
| quiet_hours_start | TIME | | Do-not-disturb start |
| quiet_hours_end | TIME | | Do-not-disturb end |
| locale | VARCHAR | | Language preference |
| created_at | TIMESTAMP | | Created |
| updated_at | TIMESTAMP | | Updated |

---

### 6.3 DeviceToken (device_tokens table)
**Purpose**: Push notification tokens
**Table Name**: `device_tokens`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Token ID |
| user_id | UUID | INDEXED | User |
| token | VARCHAR | UNIQUE INDEX | FCM/APNs push token |
| platform | VARCHAR | | ios, android, web |
| device_id | VARCHAR | | Device identifier |
| app_version | VARCHAR | | App version |
| is_active | BOOLEAN | | Token active |
| last_used | TIMESTAMP | | Last usage |
| created_at | TIMESTAMP | | Registered |
| updated_at | TIMESTAMP | | Updated |

---

### 6.4 NotificationQueue (notification_queues table)
**Purpose**: Background processing queue for notifications
**Table Name**: `notification_queues`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Queue entry ID |
| notification_id | UUID | INDEXED | Notification |
| channel | VARCHAR | | Delivery channel |
| status | VARCHAR | | pending, sent, failed |
| attempts | INT | | Delivery attempts |
| max_attempts | INT | | Max retries (default 3) |

---

## 7. CHAT/MESSAGING MODELS

### 7.1 ChatRoom (chat_rooms table)
**Purpose**: Conversation between rider and driver
**Table Name**: `chat_rooms`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Room ID |
| ride_id | UUID | UNIQUE INDEX | Associated ride |
| rider_id | UUID | | Rider |
| driver_id | UUID | | Driver |
| is_active | BOOLEAN | | Room active |
| last_message_at | TIMESTAMP | | Last message time |
| created_at | TIMESTAMP | | Created |
| updated_at | TIMESTAMP | | Updated |
| deleted_at | TIMESTAMP | | Soft delete |

---

### 7.2 ChatMessage (chat_messages table)
**Purpose**: Individual chat message
**Table Name**: `chat_messages`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Message ID |
| room_id | UUID | INDEXED | Chat room |
| sender_id | UUID | | Message sender |
| sender_type | VARCHAR | | rider, driver, system |
| type | VARCHAR | | text, image, location, voice, system |
| content | VARCHAR | | Message text |
| media_url | VARCHAR | | URL for image/voice |
| latitude | FLOAT | | Location latitude |
| longitude | FLOAT | | Location longitude |
| status | VARCHAR | | sending, sent, delivered, read, failed |
| sent_at | TIMESTAMP | | Sent time |
| delivered_at | TIMESTAMP | | Delivered time |
| read_at | TIMESTAMP | | Read time |
| created_at | TIMESTAMP | | Created |
| updated_at | TIMESTAMP | | Updated |
| deleted_at | TIMESTAMP | | Soft delete |

---

### 7.3 ChatReadReceipt (chat_read_receipts table)
**Purpose**: Track which messages have been read
**Table Name**: `chat_read_receipts`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Receipt ID |
| room_id | UUID | INDEXED | Chat room |
| user_id | UUID | | Reader |
| last_read_message_id | UUID | | Last read message |
| read_at | TIMESTAMP | | When read |
| created_at | TIMESTAMP | | Record time |
| updated_at | TIMESTAMP | | Updated |

---

### 7.4 ChatQuickReply (chat_quick_replies table)
**Purpose**: Pre-written responses for quick reply
**Table Name**: `chat_quick_replies`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Reply ID |
| category | VARCHAR | | rider, driver, both |
| message | VARCHAR | | Quick reply text |
| language | VARCHAR | | en (default) |
| order | INT | | Display order |
| is_active | BOOLEAN | | Active status |
| created_at | TIMESTAMP | | Created |

---

## 8. SAFETY & EMERGENCY MODELS

### 8.1 EmergencyContact (emergency_contacts table)
**Purpose**: User's emergency contacts
**Table Name**: `emergency_contacts`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Contact ID |
| user_id | UUID | INDEXED | User |
| name | VARCHAR | | Contact name |
| phone | VARCHAR | | Contact phone |
| relationship | VARCHAR | | spouse, parent, sibling, friend, etc. |
| priority | INT | | 1=primary, 2=secondary, etc. |
| is_active | BOOLEAN | | Active status |
| created_at | TIMESTAMP | | Added |
| updated_at | TIMESTAMP | | Updated |
| deleted_at | TIMESTAMP | | Soft delete |

---

### 8.2 SOSEvent (sos_events table)
**Purpose**: SOS emergency alert
**Table Name**: `sos_events`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Event ID |
| user_id | UUID | INDEXED | User who triggered |
| ride_id | UUID | NULLABLE INDEXED | Associated ride |
| latitude | FLOAT | | Location latitude |
| longitude | FLOAT | | Location longitude |
| address | VARCHAR | | Location address |
| status | VARCHAR | | active, resolved, false_alarm |
| resolved_at | TIMESTAMP | | Resolution time |
| resolved_by | UUID | | Who resolved |
| notes | VARCHAR | | Resolution notes |
| created_at | TIMESTAMP | | Triggered |
| updated_at | TIMESTAMP | | Updated |

---

### 8.3 SOSNotification (sos_notifications table)
**Purpose**: Notification sent for SOS event
**Table Name**: `sos_notifications`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Notification ID |
| sos_event_id | UUID | INDEXED | SOS event |
| contact_id | UUID | | Contact notified |
| notification_type | VARCHAR | | sms, push, call |
| status | VARCHAR | | pending, sent, failed, delivered |
| sent_at | TIMESTAMP | | Send time |
| error_message | VARCHAR | | Error if failed |
| created_at | TIMESTAMP | | Created |

---

### 8.4 SOSAlert (sos_alerts table)
**Purpose**: Enhanced SOS alert system
**Table Name**: `sos_alerts`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Alert ID |
| ride_id | UUID | NULLABLE INDEXED | Associated ride |
| user_id | UUID | INDEXED | User |
| user_type | VARCHAR | | rider, driver |
| status | VARCHAR | | active, resolved, false_alarm, escalated |
| latitude | FLOAT | | Alert location |
| longitude | FLOAT | | Alert location |
| address | VARCHAR | | Location address |
| reason | VARCHAR | | Alert reason |
| triggered_by | VARCHAR | | manual, auto_crash_detection, panic_button |
| resolved_by | UUID | | Who resolved |
| resolved_at | TIMESTAMP | | Resolution time |
| resolution_notes | VARCHAR | | Notes |
| notifications_sent | JSONB | | Notifications sent |
| emergency_contacts_notified | BOOLEAN | | Contacts notified |
| police_notified | BOOLEAN | | Police contacted |
| ambulance_called | BOOLEAN | | Ambulance requested |
| audio_recording_url | VARCHAR | | Recording URL |
| created_at | TIMESTAMP | | Created |
| updated_at | TIMESTAMP | | Updated |
| deleted_at | TIMESTAMP | | Soft delete |

---

### 8.5 TripShare (trip_shares table)
**Purpose**: Share live location with trusted contacts
**Table Name**: `trip_shares`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Share ID |
| ride_id | UUID | INDEXED | Shared ride |
| rider_id | UUID | | Rider sharing |
| share_token | VARCHAR | UNIQUE INDEX | Token for link |
| expires_at | TIMESTAMP | | Share expiration |
| is_active | BOOLEAN | | Currently active |
| created_at | TIMESTAMP | | Created |
| updated_at | TIMESTAMP | | Updated |

---

### 8.6 ShareRecipient (share_recipients table)
**Purpose**: Person with whom trip is shared
**Table Name**: `share_recipients`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Recipient ID |
| trip_share_id | UUID | INDEXED | Trip share |
| name | VARCHAR | | Recipient name |
| phone | VARCHAR | | Contact phone |
| email | VARCHAR | | Email address |
| notified_at | TIMESTAMP | | Notification time |
| viewed_at | TIMESTAMP | | Viewing time |
| created_at | TIMESTAMP | | Added |

---

### 8.7 SafetyCheckIn (safety_check_ins table)
**Purpose**: Periodic safety check-ins during ride
**Table Name**: `safety_check_ins`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Check-in ID |
| ride_id | UUID | INDEXED | Ride |
| user_id | UUID | | User being checked |
| scheduled_at | TIMESTAMP | | Scheduled check-in time |
| checked_in_at | TIMESTAMP | | Actual check-in time |
| is_overdue | BOOLEAN | | Not checked in on time |
| auto_triggered_sos | BOOLEAN | | Triggered SOS if missed |
| created_at | TIMESTAMP | | Created |

---

### 8.8 SafetySettings (safety_settings table)
**Purpose**: User's safety preferences
**Table Name**: `safety_settings`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Settings ID |
| user_id | UUID | UNIQUE INDEX | User |
| share_ride_by_default | BOOLEAN | | Auto-share rides |
| emergency_contact_alerts | BOOLEAN | | Notify emergency contacts |
| police_alert | BOOLEAN | | Alert police on SOS |
| ambulance_alert | BOOLEAN | | Request ambulance |
| sos_button_enabled | BOOLEAN | | SOS button active |
| crash_detection_enabled | BOOLEAN | | Auto crash detection |
| periodic_check_ins | BOOLEAN | | Enable check-ins |
| check_in_interval | INT | | Minutes between check-ins |
| created_at | TIMESTAMP | | Created |
| updated_at | TIMESTAMP | | Updated |

---

### 8.9 IncidentReport (incident_reports table)
**Purpose**: Report safety incidents
**Table Name**: `incident_reports`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Report ID |
| ride_id | UUID | INDEXED | Incident ride |
| reporter_id | UUID | | Who reported |
| reporter_type | VARCHAR | | rider, driver |
| incident_type | VARCHAR | | harassment, rash_driving, overcharging, etc. |
| severity | VARCHAR | | low, medium, high, critical |
| description | VARCHAR | | Incident details |
| evidence_urls | TEXT[] | | Photo/video URLs |
| status | VARCHAR | | open, investigating, resolved, closed |
| assigned_to | UUID | | Assigned admin |
| resolution | VARCHAR | | How it was resolved |
| resolved_at | TIMESTAMP | | Resolution time |
| created_at | TIMESTAMP | | Reported |
| updated_at | TIMESTAMP | | Updated |
| deleted_at | TIMESTAMP | | Soft delete |

---

## 9. SUPPORT & DISPUTE MODELS

### 9.1 SupportTicket (support_tickets table)
**Purpose**: Customer support tickets
**Table Name**: `support_tickets`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Ticket ID |
| ticket_number | VARCHAR | UNIQUE INDEX | User-friendly number |
| user_id | UUID | INDEXED | Who opened |
| user_type | VARCHAR | | rider, driver |
| category | VARCHAR | | payment_issue, ride_issue, safety, account, other |
| priority | VARCHAR | | low, medium, high, critical |
| status | VARCHAR | | open, in_progress, resolved, closed, escalated |
| subject | VARCHAR | | Ticket subject |
| description | VARCHAR | | Issue description |
| ride_id | UUID | NULLABLE | Related ride |
| refund_amount | FLOAT | | Refund if applicable |
| refund_status | VARCHAR | | pending, processed, rejected |
| assigned_to | UUID | | Assigned admin |
| resolution | VARCHAR | | Resolution details |
| resolved_at | TIMESTAMP | | Resolution time |
| created_at | TIMESTAMP | | Created |
| updated_at | TIMESTAMP | | Updated |
| deleted_at | TIMESTAMP | | Soft delete |

---

### 9.2 SupportTicketMessage (support_ticket_messages table)
**Purpose**: Messages in support ticket
**Table Name**: `support_ticket_messages`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Message ID |
| ticket_id | UUID | INDEXED | Ticket |
| sender_id | UUID | | Message author |
| sender_type | VARCHAR | | user, admin, system |
| message | VARCHAR | | Message content |
| is_internal | BOOLEAN | | Admin-only note |
| created_at | TIMESTAMP | | Posted |
| deleted_at | TIMESTAMP | | Soft delete |

---

### 9.3 Dispute (disputes table)
**Purpose**: Ride fare/quality disputes
**Table Name**: `disputes`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Dispute ID |
| ride_id | UUID | UNIQUE INDEX | Disputed ride |
| disputed_by | UUID | | Complainant |
| disputed_by_type | VARCHAR | | rider, driver |
| reason | VARCHAR | | route_manipulation, overcharge, behavior, other |
| description | VARCHAR | | Complaint details |
| expected_fare | FLOAT | | What they think fare should be |
| actual_fare | FLOAT | | What was charged |
| status | VARCHAR | | pending, under_review, resolved_rejected, resolved_accepted |
| admin_notes | VARCHAR | | Admin review notes |
| refund_amount | FLOAT | | Refund issued |
| resolution | VARCHAR | | Resolution details |
| resolved_by | UUID | | Resolving admin |
| resolved_at | TIMESTAMP | | Resolution time |
| created_at | TIMESTAMP | | Filed |
| updated_at | TIMESTAMP | | Updated |
| deleted_at | TIMESTAMP | | Soft delete |

---

## 10. SURGE PRICING & FARE MODELS

### 10.1 FareConfig (fare_configs table)
**Purpose**: Fare configuration by vehicle type
**Table Name**: `fare_configs`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Config ID |
| vehicle_type | VARCHAR | UNIQUE INDEX | bike, auto, car, etc. |
| base_fare | FLOAT | | Minimum charge |
| per_km_rate | FLOAT | | Cost per kilometer |
| per_min_rate | FLOAT | | Cost per minute |
| min_fare | FLOAT | | Absolute minimum |
| max_fare | FLOAT | | Absolute maximum |
| platform_fee | FLOAT | | Fixed platform fee |
| service_fee | FLOAT | | Additional service charge |
| night_multiplier | FLOAT | | Multiplier for night (11pm-6am) |
| is_active | BOOLEAN | | Currently active |
| created_at | TIMESTAMP | | Created |
| updated_at | TIMESTAMP | | Updated |

---

### 10.2 SurgePricing (surge_pricings table)
**Purpose**: Dynamic pricing zones
**Table Name**: `surge_pricings`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Zone ID |
| area_name | VARCHAR | INDEXED | Area name |
| latitude | FLOAT | | Zone center latitude |
| longitude | FLOAT | | Zone center longitude |
| radius_km | FLOAT | | Zone radius |
| multiplier | FLOAT | | Pricing multiplier (e.g., 1.5x) |
| is_active | BOOLEAN | | Currently active |
| reason | VARCHAR | | Why surge (peak_hours, weather, etc.) |
| start_time | TIMESTAMP | | Surge start |
| end_time | TIMESTAMP | | Surge end |
| created_by | UUID | | Admin who created |
| created_at | TIMESTAMP | | Created |
| updated_at | TIMESTAMP | | Updated |

---

## 11. INCENTIVES & PROMOTIONS MODELS

### 11.1 Incentive (incentives table)
**Purpose**: Driver incentive programs
**Table Name**: `incentives`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Program ID |
| title | VARCHAR | | Incentive title |
| description | VARCHAR | | Program description |
| type | VARCHAR | | weekly_target, streak, peak_hour, referral |
| start_date | TIMESTAMP | | Program start |
| end_date | TIMESTAMP | | Program end |
| target_rides | INT | | Rides to complete |
| target_hours | INT | | Hours to work |
| target_earnings | FLOAT | | Earnings target |
| reward_amount | FLOAT | | Fixed reward |
| bonus_per_ride | FLOAT | | Per-ride bonus |
| valid_vehicle_types | TEXT[] | | Eligible vehicle types |
| valid_cities | TEXT[] | | Eligible cities |
| is_active | BOOLEAN | | Currently active |
| created_at | TIMESTAMP | | Created |
| updated_at | TIMESTAMP | | Updated |
| deleted_at | TIMESTAMP | | Soft delete |

---

### 11.2 DriverIncentive (driver_incentives table)
**Purpose**: Driver progress on incentives
**Table Name**: `driver_incentives`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Record ID |
| driver_id | UUID | INDEXED | Driver |
| incentive_id | UUID | INDEXED | Program |
| progress | INT | | Current progress |
| target | INT | | Target to reach |
| status | VARCHAR | | in_progress, completed, claimed, expired |
| earned_amount | FLOAT | | Amount earned so far |
| claimed_at | TIMESTAMP | | When claimed |
| completed_at | TIMESTAMP | | When completed |
| created_at | TIMESTAMP | | Created |
| updated_at | TIMESTAMP | | Updated |

---

### 11.3 WeeklyTarget (weekly_targets table)
**Purpose**: Weekly performance targets
**Table Name**: `weekly_targets`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Target ID |
| driver_id | UUID | INDEXED | Driver |
| week_start | TIMESTAMP | INDEXED | Week start date |
| week_end | TIMESTAMP | | Week end date |
| target_rides | INT | | Target ride count |
| completed_rides | INT | | Rides completed |
| target_hours | INT | | Target hours |
| completed_hours | FLOAT | | Actual hours |
| target_earnings | FLOAT | | Earnings goal |
| actual_earnings | FLOAT | | Actual earnings |
| incentive_earned | FLOAT | | Bonus earned |
| status | VARCHAR | | in_progress, target_met, completed |
| created_at | TIMESTAMP | | Created |
| updated_at | TIMESTAMP | | Updated |

---

### 11.4 PromoCode (promo_codes table)
**Purpose**: Discount codes
**Table Name**: `promo_codes`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Code ID |
| code | VARCHAR | UNIQUE INDEX | Promo code string |
| description | VARCHAR | | What discount offers |
| discount_type | VARCHAR | | percentage or fixed |
| discount_value | FLOAT | | Discount amount/% |
| max_discount | FLOAT | | Maximum discount cap |
| min_ride_amount | FLOAT | | Minimum ride amount |
| max_uses | INT | | Total uses allowed (0=unlimited) |
| uses_count | INT | | Current uses |
| max_uses_per_user | INT | | Uses per user (default 1) |
| applicable_cities | TEXT[] | | Eligible cities |
| applicable_vehicle_types | TEXT[] | | Eligible vehicle types |
| start_date | TIMESTAMP | | Valid from |
| end_date | TIMESTAMP | | Valid until |
| is_active | BOOLEAN | | Active status |
| created_by | UUID | | Admin who created |
| created_at | TIMESTAMP | | Created |
| updated_at | TIMESTAMP | | Updated |
| deleted_at | TIMESTAMP | | Soft delete |

---

### 11.5 PromoCodeUsage (promo_code_usages table)
**Purpose**: Track code usage
**Table Name**: `promo_code_usages`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Usage ID |
| promo_code_id | UUID | INDEXED | Code |
| user_id | UUID | INDEXED | User |
| ride_id | UUID | | Ride where used |
| discount_amount | FLOAT | | Discount given |
| used_at | TIMESTAMP | | Usage time |

---

## 12. SCHEDULED RIDE MODEL

### 12.1 ScheduledRide (scheduled_rides table)
**Purpose**: Pre-scheduled rides
**Table Name**: `scheduled_rides`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Schedule ID |
| rider_id | UUID | INDEXED | Rider |
| pickup_lat | FLOAT | | Pickup latitude |
| pickup_lng | FLOAT | | Pickup longitude |
| pickup_address | VARCHAR | | Pickup address |
| dropoff_lat | FLOAT | | Dropoff latitude |
| dropoff_lng | FLOAT | | Dropoff longitude |
| dropoff_address | VARCHAR | | Dropoff address |
| vehicle_type | VARCHAR | | bike, auto, car, etc. |
| scheduled_at | TIMESTAMP | INDEXED | Scheduled time |
| status | VARCHAR | | pending, notified, assigned, completed, cancelled |
| notes | VARCHAR | | Special instructions |
| ride_id | UUID | NULLABLE | Actual ride once created |
| notification_sent_at | TIMESTAMP | | Reminder sent |
| created_at | TIMESTAMP | | Created |
| updated_at | TIMESTAMP | | Updated |
| deleted_at | TIMESTAMP | | Soft delete |

---

## 13. ADMIN & AUDIT MODELS

### 13.1 Admin (admins table)
**Purpose**: Admin user accounts
**Table Name**: `admins`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Admin ID |
| user_id | UUID | UNIQUE INDEX | Associated user |
| role | VARCHAR | | super_admin, admin, support, finance, operations |
| status | VARCHAR | | active, inactive, suspended |
| department | VARCHAR | | Admin's department |
| permissions | TEXT[] | | Capability permissions |
| last_login_at | TIMESTAMP | | Last login |
| login_ip | VARCHAR | | Last login IP |
| created_by | UUID | | Who created this admin |
| created_at | TIMESTAMP | | Created |
| updated_at | TIMESTAMP | | Updated |
| deleted_at | TIMESTAMP | | Soft delete |

**Roles**: super_admin, admin, support, finance, operations

---

### 13.2 AdminActivityLog (admin_activity_logs table)
**Purpose**: Audit trail of admin actions
**Table Name**: `admin_activity_logs`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Log ID |
| admin_id | UUID | INDEXED | Admin |
| action | VARCHAR | | Action performed |
| entity_type | VARCHAR | | driver, ride, user, etc. |
| entity_id | UUID | | Related resource ID |
| old_values | JSONB | | Previous values |
| new_values | JSONB | | Updated values |
| description | VARCHAR | | Action description |
| ip | VARCHAR | | Admin's IP |
| user_agent | VARCHAR | | Browser info |
| created_at | TIMESTAMP | | Log time |

---

### 13.3 AuditLog (audit_logs table)
**Purpose**: System-wide audit trail
**Table Name**: `audit_logs`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Log ID |
| user_id | UUID | NULLABLE INDEXED | User involved |
| user_type | VARCHAR | | rider, driver, admin, system |
| action | VARCHAR | INDEXED | Action (ride_requested, payment_processed, etc.) |
| entity_type | VARCHAR | INDEXED | Resource type |
| entity_id | VARCHAR | INDEXED | Resource ID |
| old_values | JSONB | | Before values |
| new_values | JSONB | | After values |
| ip_address | VARCHAR | | Request IP |
| user_agent | VARCHAR | | Browser info |
| device_id | VARCHAR | | Device |
| request_id | VARCHAR | | Request ID |
| status | VARCHAR | | success, failed, denied |
| error_message | VARCHAR | | Error if any |
| severity | VARCHAR | | info, warning, critical |
| created_at | TIMESTAMP | | Log time |
| deleted_at | TIMESTAMP | | Soft delete |

**Severity Levels**: info, warning, critical

---

### 13.4 SystemSettings (system_settings table)
**Purpose**: Application configuration
**Table Name**: `system_settings`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | Setting ID |
| key | VARCHAR | UNIQUE INDEX | Setting key |
| value | VARCHAR | | Setting value |
| data_type | VARCHAR | | string, integer, boolean, json |
| description | VARCHAR | | What this setting does |
| is_editable | BOOLEAN | | Can be changed via UI |
| created_at | TIMESTAMP | | Created |
| updated_at | TIMESTAMP | | Updated |

---

## 14. CITY MODEL

### 14.1 City (cities table)
**Purpose**: Operational cities
**Table Name**: `cities`

| Field | Type | Key | Description |
|-------|------|-----|-------------|
| id | UUID | PRIMARY KEY | City ID |
| name | VARCHAR | | City name |
| state | VARCHAR | | State/province |
| country | VARCHAR | | Country (default: India) |
| currency | VARCHAR | | Currency code (default: INR) |
| is_active | BOOLEAN | | Operating in city |
| latitude | FLOAT | | City center latitude |
| longitude | FLOAT | | City center longitude |
| radius_km | FLOAT | | Service radius |
| timezone | VARCHAR | | Timezone (default: Asia/Kolkata) |
| created_at | TIMESTAMP | | Added |
| updated_at | TIMESTAMP | | Updated |

---

## KEY RELATIONSHIPS & FOREIGN KEYS

### Primary User-Related Relationships
- **PublicUser** → Driver (1-to-1 via user_id)
- **PublicUser** → Admin (1-to-1 via user_id)
- **PublicUser** → Device (1-to-many)
- **PublicUser** → OTP (1-to-many via phone)
- **PublicUser** → RefreshToken (1-to-many)
- **PublicUser** → NotificationPreference (1-to-1)
- **PublicUser** → Wallet (1-to-1)
- **PublicUser** → EmergencyContact (1-to-many)
- **PublicUser** → DeviceToken (1-to-many)

### Driver-Related Relationships
- **Driver** → Vehicle (1-to-many)
- **Driver** → DriverLocation (1-to-1)
- **Driver** → DriverDocument (1-to-many)
- **Driver** → DriverEarnings (1-to-1)
- **Driver** → DriverIncentive (1-to-many via DriverIncentive)
- **Driver** → Rating (1-to-many via DriverID)
- **Driver** → DriverRatingSummary (1-to-1)

### Ride-Related Relationships
- **Ride** → PublicUser (Rider via RiderID)
- **Ride** → Driver (1-to-many via DriverID)
- **Ride** → Vehicle (via VehicleID)
- **Ride** → Payment (1-to-1)
- **Ride** → Rating (1-to-1 via RideID)
- **Ride** → RideLocation (1-to-many)
- **Ride** → RideRequestLog (1-to-many)
- **Ride** → RideMatch (1-to-many)
- **Ride** → RideStatusLog (1-to-many)
- **Ride** → SupportTicket (1-to-many)
- **Ride** → Dispute (1-to-1)
- **Ride** → ScheduledRide (Scheduled ride becomes actual ride)
- **Ride** → ChatRoom (1-to-1)

### Financial Relationships
- **Payment** → Ride (1-to-1)
- **Payment** → PaymentMethod (via Method type)
- **Transaction** → Wallet (via user_id)
- **Commission** → Ride (1-to-1)
- **Commission** → Driver (1-to-many)
- **Withdrawal** → Driver (1-to-many)
- **Invoice** → Ride (1-to-1)
- **Invoice** → Payment (1-to-1)
- **LedgerEntry** → LedgerAccount (many-to-1)
- **PromoCodeUsage** → PromoCode (many-to-1)
- **PromoCodeUsage** → Ride (many-to-1)

### Notification Relationships
- **Notification** → PublicUser (1-to-many)
- **NotificationQueue** → Notification (1-to-many)

### Safety Relationships
- **SOSEvent** → PublicUser (1-to-many)
- **SOSEvent** → Ride (1-to-many via RideID)
- **SOSNotification** → SOSEvent (1-to-many)
- **SOSNotification** → EmergencyContact (via ContactID)
- **SOSAlert** → Ride (1-to-many)
- **TripShare** → Ride (1-to-1)
- **ShareRecipient** → TripShare (1-to-many)
- **SafetyCheckIn** → Ride (1-to-many)

### Support Relationships
- **SupportTicket** → PublicUser (1-to-many)
- **SupportTicket** → Ride (1-to-many)
- **SupportTicketMessage** → SupportTicket (1-to-many)
- **Dispute** → Ride (1-to-1)

### Admin Relationships
- **AdminActivityLog** → Admin (1-to-many)
- **Admin** → PublicUser (1-to-1 via user_id)

---

## DATABASE STATISTICS

**Total Tables**: 59+
**Total Models**: Comprehensive coverage of all ride-sharing functions
**Primary Key**: All tables use UUID
**Database**: PostgreSQL
**ORM**: GORM (Go)
**Soft Deletes**: Enabled on most tables via `deleted_at`

---

## INDEXING STRATEGY

**Indexed Columns** (High-Frequency Queries):
- Foreign keys and relationships
- Status columns (ride_status, payment_status, etc.)
- User and driver IDs
- Timestamps (created_at, requested_at, etc.)
- Unique identifiers (unique indexes on codes, numbers, etc.)
- Location-based queries (latitude/longitude for geospatial)

---

## DATA TYPES SUMMARY

- **UUID**: Primary keys and foreign keys
- **VARCHAR/TEXT**: Strings, addresses, descriptions
- **FLOAT**: Monetary values, coordinates, ratings
- **INT**: Counts, durations, percentages
- **BOOLEAN**: Status flags
- **TIMESTAMP**: Date/time tracking
- **TEXT[]**: Array fields (languages, permitted cities, etc.)
- **JSONB**: Flexible data (metadata, settings, old/new values)

---

## SECURITY & PRIVACY NOTES

- **Password Hashes**: Not exposed in JSON responses
- **Aadhaar**: Masked in JSON, stored encrypted
- **Card Numbers**: Encrypted at rest, last 4 exposed for display
- **UPI Details**: Bank account and IFSC encrypted
- **Soft Deletes**: Data marked deleted_at, not physically removed
- **Audit Trails**: All critical actions logged in AuditLog
- **PII Access**: Logged with critical severity

---

This comprehensive documentation covers all 59+ tables in the Rapido ride-sharing backend system, providing complete reference for developers, database administrators, and system architects.
