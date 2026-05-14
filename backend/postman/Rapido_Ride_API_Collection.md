# Rapido Ride API - Postman Collection

## Environment Variables

Set these in Postman Environment:

```json
{
  "baseUrl": "http://localhost:8080",
  "riderPhone": "9876543210",
  "driverPhone": "9876543211",
  "riderAccessToken": "",
  "driverAccessToken": "",
  "adminAccessToken": "",
  "rideId": "",7
  "driverId": "",
  "riderId": ""
}
```

## Authentication Flow

### 1. Request OTP (Rider)
```http
POST {{baseUrl}}/api/v1/auth/otp/request
Content-Type: application/json

{
  "phone": "{{riderPhone}}"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "phone": "******3210",
    "expires_in": 300
  }
}
```

### 2. Verify OTP & Get Tokens (Rider)
```http
POST {{baseUrl}}/api/v1/auth/otp/verify
Content-Type: application/json

{
  "phone": "{{riderPhone}}",
  "email": "rider@example.com",
  "otp": "123456",
  "name": "Test Rider",
  "user_type": "rider"
}
```

**Save these variables from response:**
- `access_token` → `riderAccessToken`
- `user.id` → `riderId`

### 3. Request OTP (Driver)
```http
POST {{baseUrl}}/api/v1/auth/otp/request
Content-Type: application/json

{
  "phone": "{{driverPhone}}"
}
```

### 4. Verify OTP & Get Tokens (Driver)
```http
POST {{baseUrl}}/api/v1/auth/otp/verify
Content-Type: application/json

{
  "phone": "{{driverPhone}}",
  "email": "driver@example.com",
  "otp": "123456",
  "name": "Test Driver",
  "user_type": "driver"
}
```

**Save these variables from response:**
- `access_token` → `driverAccessToken`
- `user.id` → `driverId`

## Ride Endpoints

### 5. Get User Profile (Rider)
```http
GET {{baseUrl}}/api/v1/auth/profile
Authorization: Bearer {{riderAccessToken}}
```

### 6. Request New Ride
```http
POST {{baseUrl}}/api/v1/rides
Authorization: Bearer {{riderAccessToken}}
Content-Type: application/json

{
  "vehicle_type": "bike",
  "pickup_lat": 19.0760,
  "pickup_lng": 72.8777,
  "pickup_address": "Mumbai, Andheri East",
  "dropoff_lat": 19.0178,
  "dropoff_lng": 72.8478,
  "dropoff_address": "Mumbai, Bandra West",
  "payment_method": "wallet",
  "promo_code": "RAPIDO50",
  "preferences": {
    "female_driver": false,
    "ac": false,
    "luggage": false
  }
}
```

**Save `data.id` → `rideId`**

### 7. Get Active Ride
```http
GET {{baseUrl}}/api/v1/rides/active
Authorization: Bearer {{riderAccessToken}}
```

### 8. Get Ride Details
```http
GET {{baseUrl}}/api/v1/rides/{{rideId}}
Authorization: Bearer {{riderAccessToken}}
```

### 9. Estimate Fare
```http
GET {{baseUrl}}/api/v1/rides/estimate?pickup_lat=19.0760&pickup_lng=72.8777&dropoff_lat=19.0178&dropoff_lng=72.8478&vehicle_type=bike
Authorization: Bearer {{riderAccessToken}}
```

### 10. Get Nearby Drivers
```http
GET {{baseUrl}}/api/v1/drivers/nearby?lat=19.0760&lng=72.8777&vehicle_type=bike
Authorization: Bearer {{riderAccessToken}}
```

## Driver Endpoints

### 11. Driver Go Online
```http
POST {{baseUrl}}/api/v1/drivers/online
Authorization: Bearer {{driverAccessToken}}
Content-Type: application/json

{
  "lat": 19.0760,
  "lng": 72.8777,
  "vehicle_type": "bike"
}
```

### 12. Driver Update Location
```http
POST {{baseUrl}}/api/v1/drivers/location
Authorization: Bearer {{driverAccessToken}}
Content-Type: application/json

{
  "lat": 19.0760,
  "lng": 72.8777
}
```

### 13. Driver Accept Ride
```http
POST {{baseUrl}}/api/v1/rides/{{rideId}}/accept
Authorization: Bearer {{driverAccessToken}}
```

### 14. Driver Arrived at Pickup
```http
POST {{baseUrl}}/api/v1/rides/{{rideId}}/arrived
Authorization: Bearer {{driverAccessToken}}
```

### 15. Start Ride (with OTP)
```http
POST {{baseUrl}}/api/v1/rides/{{rideId}}/start
Authorization: Bearer {{driverAccessToken}}
Content-Type: application/json

{
  "otp": "123456"
}
```

### 16. Complete Ride
```http
POST {{baseUrl}}/api/v1/rides/{{rideId}}/complete
Authorization: Bearer {{driverAccessToken}}
Content-Type: application/json

{
  "final_lat": 19.0178,
  "final_lng": 72.8478
}
```

## Ride Tracking & Status

### 17. Track Ride
```http
GET {{baseUrl}}/api/v1/rides/{{rideId}}/track
Authorization: Bearer {{riderAccessToken}}
```

### 18. Get Ride ETA
```http
GET {{baseUrl}}/api/v1/rides/{{rideId}}/eta
Authorization: Bearer {{riderAccessToken}}
```

### 19. Cancel Ride (Rider)
```http
POST {{baseUrl}}/api/v1/rides/{{rideId}}/cancel
Authorization: Bearer {{riderAccessToken}}
Content-Type: application/json

{
  "reason": "Changed my mind"
}
```

### 20. Reject Ride (Driver)
```http
POST {{baseUrl}}/api/v1/rides/{{rideId}}/reject
Authorization: Bearer {{driverAccessToken}}
Content-Type: application/json

{
  "reason": "Too far away"
}
```

## Payment & Wallet

### 21. Get Wallet Balance
```http
GET {{baseUrl}}/api/v1/wallet
Authorization: Bearer {{riderAccessToken}}
```

### 22. Add Money to Wallet
```http
POST {{baseUrl}}/api/v1/wallet/add-money
Authorization: Bearer {{riderAccessToken}}
Content-Type: application/json

{
  "amount": 500,
  "payment_method": "card"
}
```

### 23. Process Payment for Ride
```http
POST {{baseUrl}}/api/v1/payments/rides/{{rideId}}/pay
Authorization: Bearer {{riderAccessToken}}
Content-Type: application/json

{
  "payment_method": "wallet"
}
```

## Ride History

### 24. Get Ride History
```http
GET {{baseUrl}}/api/v1/rides/history?page=1&per_page=10
Authorization: Bearer {{riderAccessToken}}
```

### 25. Get Driver Earnings
```http
GET {{baseUrl}}/api/v1/drivers/earnings
Authorization: Bearer {{driverAccessToken}}
```

### 26. Get Driver Stats
```http
GET {{baseUrl}}/api/v1/drivers/stats
Authorization: Bearer {{driverAccessToken}}
```

## Rating & Reviews

### 27. Submit Rating
```http
POST {{baseUrl}}/api/v1/rides/{{rideId}}/rate
Authorization: Bearer {{riderAccessToken}}
Content-Type: application/json

{
  "rating": 5,
  "comment": "Great ride!",
  "driver_rating": 5,
  "app_rating": 5
}
```

### 28. Get Driver Reviews
```http
GET {{baseUrl}}/api/v1/drivers/{{driverId}}/reviews
Authorization: Bearer {{riderAccessToken}}
```

## Emergency & Support

### 29. Add Emergency Contact
```http
POST {{baseUrl}}/api/v1/auth/emergency-contacts
Authorization: Bearer {{riderAccessToken}}
Content-Type: application/json

{
  "name": "John Doe",
  "phone": "9876543212",
  "relationship": "friend",
  "priority": 1
}
```

### 30. Trigger SOS
```http
POST {{baseUrl}}/api/v1/sos/trigger
Authorization: Bearer {{riderAccessToken}}
Content-Type: application/json

{
  "lat": 19.0760,
  "lng": 72.8777,
  "address": "Mumbai, Andheri East",
  "ride_id": "{{rideId}}"
}
```

### 31. Create Support Ticket
```http
POST {{baseUrl}}/api/v1/users/support/tickets
Authorization: Bearer {{riderAccessToken}}
Content-Type: application/json

{
  "subject": "Issue with ride",
  "description": "Driver was late",
  "category": "ride_issue"
}
```

## Admin Endpoints

### 32. Get Dashboard Stats
```http
GET {{baseUrl}}/api/v1/admin/dashboard
Authorization: Bearer {{adminAccessToken}}
```

### 33. Get All Rides
```http
GET {{baseUrl}}/api/v1/admin/rides?page=1&per_page=20
Authorization: Bearer {{adminAccessToken}}
```

### 34. Get All Users
```http
GET {{baseUrl}}/api/v1/admin/users?page=1&per_page=20
Authorization: Bearer {{adminAccessToken}}
```

### 35. Get All Drivers
```http
GET {{baseUrl}}/api/v1/admin/drivers
Authorization: Bearer {{adminAccessToken}}
```

## Testing Flow

### Complete Ride Flow:
1. **Setup**: Run steps 1-4 to get tokens for rider and driver
2. **Driver Online**: Run step 11 to make driver available
3. **Request Ride**: Run step 6 to create a ride request
4. **Accept Ride**: Run step 13 for driver to accept
5. **Track Progress**: Run steps 17-18 for real-time tracking
6. **Complete Journey**: Run steps 14-16 for ride completion
7. **Rate & Pay**: Run steps 27 and 23 for payment and rating

## Troubleshooting: "rider not found or inactive" Error

### 🚨 **Quick Fix:**

If you get "rider not found or inactive" error, the rider account doesn't exist or has wrong role. Fix it with this SQL:

```sql
-- Update existing user to be an active rider
UPDATE public_users 
SET role = 'rider', is_active = true, name = 'Test Rider', email = 'rider@example.com'
WHERE phone = '9876543210';

-- Or create a new rider if none exists
INSERT INTO public_users (id, name, email, phone, role, is_active, provider, created_at, updated_at)
VALUES (
  gen_random_uuid(),
  'Test Rider',
  'rider@example.com', 
  '9876543210',
  'rider',
  true,
  'local',
  NOW(),
  NOW()
) ON CONFLICT (phone) DO UPDATE SET 
  role = 'rider',
  is_active = true,
  name = 'Test Rider',
  email = 'rider@example.com';
```

### 🔍 **Verification Steps:**

1. **Check User Exists:**
```http
GET {{baseUrl}}/api/v1/auth/profile
Authorization: Bearer {{riderAccessToken}}
```

2. **Verify User Role:**
The response should show:
```json
{
  "success": true,
  "data": {
    "id": "...",
    "role": "rider",
    "is_active": true,
    "phone": "9876543210"
  }
}
```

3. **If Profile Fails:**
- User doesn't exist → Create with SQL above
- Wrong role → Update role to "rider"
- Inactive → Set is_active = true

### 🎯 **Complete Fix Process:**

1. **Run SQL Update** (using pgAdmin or psql)
2. **Request New OTP** for rider phone
3. **Verify OTP** to get fresh token
4. **Test Profile** to confirm it works
5. **Request Ride** - should work now!

### 📱 **Test Phone Numbers:**
- **Rider**: `9876543210` (must have role="rider")
- **Driver**: `9876543211` (must have role="driver")

### 🔧 **Common Issues:**
- Using driver token for rider endpoints
- User exists but has wrong role
- User exists but is inactive
- Token expired - get new one

## Debugging Ride Request Issues

### 🚨 **"No active ride" - Step by Step Debug:**

#### **Step 1: Verify Rider Account**
```http
GET {{baseUrl}}/api/v1/auth/profile
Authorization: Bearer {{riderAccessToken}}
```
**Expected:** Role should be "rider" and is_active should be true

#### **Step 2: Check Ride History**
```http
GET {{baseUrl}}/api/v1/rides/history?page=1&per_page=5
Authorization: Bearer {{riderAccessToken}}
```
**Check:** If recent rides exist and their status

#### **Step 3: Test Fare Estimation**
```http
GET {{baseUrl}}/api/v1/rides/estimate?pickup_lat=19.0760&pickup_lng=72.8777&dropoff_lat=19.0178&dropoff_lng=72.8478&vehicle_type=bike
Authorization: Bearer {{riderAccessToken}}
```
**Expected:** Should return fare breakdown

#### **Step 4: Check Available Drivers**
```http
GET {{baseUrl}}/api/v1/drivers/nearby?lat=19.0760&lng=72.8777&vehicle_type=bike
Authorization: Bearer {{riderAccessToken}}
```
**Expected:** Should list available drivers

#### **Step 5: Make Sure Driver is Online**
```http
POST {{baseUrl}}/api/v1/drivers/online
Authorization: Bearer {{driverAccessToken}}
Content-Type: application/json

{
  "lat": 19.0760,
  "lng": 72.8777,
  "vehicle_type": "bike"
}
```

#### **Step 6: Request New Ride**
```http
POST {{baseUrl}}/api/v1/rides
Authorization: Bearer {{riderAccessToken}}
Content-Type: application/json

{
  "vehicle_type": "bike",
  "pickup_lat": 19.0760,
  "pickup_lng": 72.8777,
  "pickup_address": "Mumbai, Andheri East",
  "dropoff_lat": 19.0178,
  "dropoff_lng": 72.8478,
  "dropoff_address": "Mumbai, Bandra West",
  "payment_method": "wallet"
}
```
**Save:** `data.id` → `rideId`

#### **Step 7: Check Ride Status Immediately**
```http
GET {{baseUrl}}/api/v1/rides/{{rideId}}
Authorization: Bearer {{riderAccessToken}}
```
**Expected:** Status should be "requested" or "driver_assigned"

#### **Step 8: Get Active Ride**
```http
GET {{baseUrl}}/api/v1/rides/active
Authorization: Bearer {{riderAccessToken}}
```

### 🎯 **Common Ride Request Failures:**

#### **1. No Available Drivers**
- **Error:** "no_driver_found" after timeout
- **Fix:** Make sure driver is online and nearby

#### **2. Invalid Payment Method**
- **Error:** "payment_method not supported"
- **Fix:** Use "wallet" or ensure payment method exists

#### **3. Invalid Coordinates**
- **Error:** "invalid location"
- **Fix:** Use valid Mumbai coordinates

#### **4. Rider Has Active Ride**
- **Error:** "you already have an active ride"
- **Fix:** Complete or cancel existing ride

#### **5. Surge Pricing Active**
- **Error:** "surge pricing active"
- **Fix:** Accept surge or try different location

### 🔍 **Backend Debugging:**

Check these database tables if ride request fails:

```sql
-- Check recent rides
SELECT id, rider_id, status, created_at FROM rides 
WHERE rider_id = 'YOUR_RIDER_ID' 
ORDER BY created_at DESC LIMIT 5;

-- Check driver availability
SELECT id, is_online, vehicle_type, latitude, longitude 
FROM drivers WHERE is_online = true;

-- Check ride matches
SELECT ride_id, driver_id, status, notified_at 
FROM ride_matches WHERE ride_id = 'YOUR_RIDE_ID';
```

### ⚡ **Quick Test Sequence:**

1. **Driver Online** → Step 5
2. **Request Ride** → Step 6 
3. **Check Status** → Step 7 (should be "requested")
4. **Driver Accept** → Use driver token to accept
5. **Check Active** → Step 8 (should now return ride)

### Error Testing:
- **Invalid OTP**: Try wrong OTP in step 2/4
- **Unauthorized**: Remove Authorization header
- **Invalid Location**: Use invalid coordinates
- **Duplicate Ride**: Try to request ride while having active ride
- **Driver Offline**: Try to accept ride when driver is offline

## Test Data

### Sample Coordinates (Mumbai):
- **Andheri East**: 19.0760, 72.8777
- **Bandra West**: 19.0178, 72.8478
- **BKC**: 19.0668, 72.8739
- **Worli**: 19.0170, 72.8288
- **Colaba**: 18.9219, 72.8347

### Sample OTPs:
- Development: `123456`
- Check Redis/Console for actual OTPs in production

## Headers to Include:

For all authenticated requests:
```
Authorization: Bearer {{accessToken}}
Content-Type: application/json
Accept: application/json
```

## Common Response Codes:

- `200` - Success
- `201` - Created (ride request, user registration)
- `400` - Bad Request (invalid data, validation errors)
- `401` - Unauthorized (invalid/missing token)
- `403` - Forbidden (insufficient permissions)
- `404` - Not Found (resource doesn't exist)
- `409` - Conflict (duplicate resource, active ride exists)
- `429` - Too Many Requests (rate limited)
- `500` - Internal Server Error

## Debugging Tips:

1. **Check Console**: Backend logs show actual OTPs and errors
2. **Verify Tokens**: Use `/api/v1/auth/profile` to validate tokens
3. **Check Ride Status**: Use `/api/v1/rides/active` to see current ride
4. **Database**: Check `public_users`, `rides`, `drivers` tables
5. **Redis**: Check cache for OTPs and ride states

## Rate Limits:

- **OTP Request**: 1 per minute per phone
- **Ride Request**: 5 per minute per user
- **Payment**: 3 per minute per user
- **General**: 100 requests per minute per IP
