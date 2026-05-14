# Driver Ride Access Workflow

## 🚕 How Drivers Get & Accept Rides

Once a rider creates a ride (like the one just created), the system automatically notifies nearby drivers in **3 progressive waves**:

### **Wave-Based Driver Dispatch System**

| Wave | Search Radius | Time | Max Drivers | Status |
|------|---------------|------|------------|--------|
| Wave 1 | 3 km | 5 sec | 3 drivers | Nearest first |
| Wave 2 | 5 km | 5 sec | 5 drivers | If Wave 1 fails |
| Wave 3 | 8 km | 10 sec | 10 drivers | If Wave 2 fails |
| Fallback | 12 km | 15 sec | 10 drivers | Last resort |

---

## **Real-Time Notifications**

### **1. WebSocket Push Notification**
When a ride is requested, the system sends real-time notifications to eligible drivers:

```json
{
  "type": "ride_offer",
  "data": {
    "ride_id": "1e05f1cc-098c-4dc7-be09-ff42d9902802",
    "vehicle_type": "bike",
    "pickup": {
      "address": "Mumbai, Andheri East",
      "lat": 19.076,
      "lng": 72.8777
    } ,
    "dropoff": {
      "address": "Mumbai, Bandra West",
      "lat": 19.0178,
      "lng": 72.8478
    },
    "estimated_fare": 108,
    "distance_km": 7.19,
    "eta_minutes": 15,
    "stage": "wave_1"
  }
}
```

### **2. Driver Selection Criteria**
The system uses **multi-factor scoring** to select the best drivers:

- **Distance** (40%) - Closest drivers get priority
- **Rating** (30%) - Higher-rated drivers prioritized
- **Acceptance Rate** (30%) - Reliable drivers preferred
- **Idle Time** - Active drivers prioritized over inactive
- **Recent Notifications** - Avoid spamming same driver
- **Vehicle Type Match** - Match rider preferences
Authorization: Bearer <driver_jwt_token>

Response:
{
  "success": true,
  "data": {
    "id": "1e05f1cc-098c-4dc7-be09-ff42d9902802",
    "status": "requested",  // or "driver_assigned", "ongoing", etc.
    "rider": {
      "name": "John Doe",
      "phone": "9876543210",
      "rating": 4.8
    },
    "pickup": {...},
    "dropoff": {...},
    "estimated_fare": 108
  }
}
```

### **2. Accept a Ride**
```
POST /api/v1/rides/{ride_id}/accept
Authorization: Bearer <driver_jwt_token>

Response:
{
  "success": true,
  "message": "Ride accepted",
  "data": {
    "id": "1e05f1cc-098c-4dc7-be09-ff42d9902802",
    "status": "driver_assigned",
    "driver_id": "driver_uuid_here",
    "ride_otp": "7168"  // To verify rider
  }
}
```

### **3. Reject a Ride**
```
POST /api/v1/rides/{ride_id}/reject
Authorization: Bearer <driver_jwt_token>
Content-Type: application/json

Body:
{
  "reason": "too_far"  // Optional
}

Response:
{
  "success": true,
  "message": "Ride rejected"
}
```

### **4. Mark as Arrived**
```
POST /api/v1/rides/{ride_id}/arrived
Authorization: Bearer <driver_jwt_token>

Response:
{
  "success": true,
  "data": {
    "status": "driver_arrived",
    "arrived_at": "2026-05-13T09:00:00Z"
  }
}
```

### **5. Start the Ride**
```
POST /api/v1/rides/{ride_id}/start
Authorization: Bearer <driver_jwt_token>

Response:
{
  "success": true,
  "data": {
    "status": "ongoing",
    "started_at": "2026-05-13T09:02:00Z"
  }
}
```

### **6. Complete the Ride**
```
POST /api/v1/rides/{ride_id}/complete
Authorization: Bearer <driver_jwt_token>
Content-Type: application/json

Body:
{
  "actual_distance": 7.25,  // km
  "actual_duration": 18      // minutes
}

Response:
{
  "success": true,
  "data": {
    "status": "completed",
    "final_fare": 110,
    "completed_at": "2026-05-13T09:20:00Z"
  }
}
```

### **7. Get Ride History**
```
GET /api/v1/rides/history?limit=10&offset=0
Authorization: Bearer <driver_jwt_token>

Response:
{
  "success": true,
  "data": [
    {
      "id": "ride_id_1",
      "status": "completed",
      "rider": {...},
      "pickup": {...},
      "dropoff": {...},
      "fare": 110,
      "rating": 5,
      "completed_at": "2026-05-13T09:20:00Z"
    }
  ]
}
```

---

## **Testing: How to Simulate a Driver**

### **Step 1: Create a Rider Account**
```bash
POST http://localhost:8081/api/v1/auth/otp/request
{
  "phone": "9876543210"
}

# Verify OTP (check logs or use 1111 for mock)
POST http://localhost:8081/api/v1/auth/otp/verify
{
  "phone": "9876543210",
  "otp": "1111"
}

# Response includes rider JWT token
# Store access_token for rides
```

### **Step 2: Create a Driver Account**
```bash
# Login as same user or different user
POST http://localhost:8081/api/v1/auth/otp/request
{
  "phone": "9999999999"  # Different phone
}

POST http://localhost:8081/api/v1/auth/otp/verify
{
  "phone": "9999999999",
  "otp": "1111"
}

# Register as driver
POST http://localhost:8081/api/v1/drivers/register
Authorization: Bearer <driver_jwt>
{
  "license_number": "DL001",
  "license_expiry": "2028-05-13",
  "rc_number": "RC001",
  "vehicle_type": "bike",
  "plate_number": "MH01AB1234"
}
```

### **Step 3: Rider Requests a Ride**
```bash
POST http://localhost:8081/api/v1/rides
Authorization: Bearer <rider_jwt>
{
  "vehicle_type": "bike",
  "pickup_lat": 19.076,
  "pickup_lng": 72.8777,
  "pickup_address": "Mumbai, Andheri East",
  "dropoff_lat": 19.0178,
  "dropoff_lng": 72.8478,
  "dropoff_address": "Mumbai, Bandra West",
  "payment_method": "wallet"
}
```

### **Step 4: Driver Accepts the Ride**
```bash
POST http://localhost:8081/api/v1/rides/1e05f1cc-098c-4dc7-be09-ff42d9902802/accept
Authorization: Bearer <driver_jwt>

# Response: Ride status changes to "driver_assigned"
```

---

## **Real-Time Updates via WebSocket**

### **Driver Receives Ride Offer**
```json
{
  "type": "ride_offer",
  "event": "ride_offer",
  "data": {
    "ride_id": "1e05f1cc-098c-4dc7-be09-ff42d9902802",
    "vehicle_type": "bike",
    ...
  }
}
```

### **Driver Notified of Status Changes**
```json
{
  "type": "ride_status_update",
  "ride_id": "1e05f1cc-098c-4dc7-be09-ff42d9902802",
  "status": "driver_assigned",
  "message": "You accepted the ride"
}
```

### **Rider Gets Live Updates**
```json
{
  "type": "ride_status_update",
  "ride_id": "1e05f1cc-098c-4dc7-be09-ff42d9902802",
  "status": "driver_assigned",
  "driver": {
    "name": "Raj Kumar",
    "rating": 4.9,
    "vehicle": "Bike - MH01AB1234"
  }
}
```

---

## **Workflow Diagram**

```
┌─────────────────────────────────────────────────────────────┐
│ 1. Rider requests a ride                                     │
│    POST /api/v1/rides                                       │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│ 2. System initiates Wave-based matching                      │
│    - Wave 1: 3km radius, 5 sec, 3 drivers                  │
│    - Wave 2: 5km radius, 5 sec, 5 drivers                  │
│    - Wave 3: 8km radius, 10 sec, 10 drivers                │
│    - Fallback: 12km radius, 15 sec, 10 drivers             │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│ 3. Push notification sent to selected drivers via WebSocket │
│    - Real-time ride offer with fare, pickup, dropoff        │
└────────────────────┬────────────────────────────────────────┘
                     │
        ┌────────────┴────────────┐
        │                         │
        ▼                         ▼
   ┌──────────┐            ┌──────────┐
   │  ACCEPT  │            │  REJECT  │
   └─────┬────┘            └────┬─────┘
         │                      │
         ▼                      ▼
    ┌─────────┐          ┌──────────────┐
    │ Driver  │          │ Next driver  │
    │ Assigned│          │ in queue     │
    │ Status: │          │ gets offer   │
    │driver_  │          └──────────────┘
    │assigned │
    └────┬────┘
         │
         ▼
┌──────────────────────────────────────┐
│ 4. Driver Updates Location           │
│    - "Driver has arrived" (5 min)    │
│    POST /api/v1/rides/{id}/arrived   │
└────────────┬─────────────────────────┘
             │
             ▼
   ┌──────────────────┐
   │ Rider boards     │
   │ Verifies OTP     │
   └────────┬─────────┘
            │
            ▼
┌──────────────────────────────────────┐
│ 5. Driver Starts Ride                │
│    POST /api/v1/rides/{id}/start     │
│    Status: "ongoing"                 │
└────────────┬─────────────────────────┘
             │
             ▼
  ┌─────────────────────┐
  │ Live location updates
  │ via WebSocket       │
  └────────┬────────────┘
           │
           ▼
┌──────────────────────────────────────┐
│ 6. Driver Completes Ride             │
│    POST /api/v1/rides/{id}/complete  │
│    Final fare calculated             │
│    Status: "completed"               │
└──────────────────────────────────────┘
```

---

## **Key Features**

✅ **Intelligent Matching** - Score-based driver selection  
✅ **Progressive Expansion** - Expand search radius if no drivers found  
✅ **Real-Time Notifications** - WebSocket push notifications  
✅ **Automatic Matching** - No manual intervention needed  
✅ **Retry Logic** - 2nd pass with expanded radius (wave_retry)  
✅ **Fallback Handling** - "No driver found" after all waves  
✅ **Rider Updates** - Real-time status updates to both parties  
✅ **Idempotency** - Prevent duplicate requests  

---

## **Status Transitions**

```
requested 
    ↓ (Driver accepts)
driver_assigned 
    ↓ (Driver arrives)
driver_arrived 
    ↓ (Driver starts)
ongoing 
    ↓ (Ride complete)
completed

(Any state) → cancelled (if either party cancels)
requested → no_driver_found (after all waves expire)
```
