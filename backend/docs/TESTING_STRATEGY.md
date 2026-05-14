# Testing Strategy

This document defines comprehensive testing approach for Rapido backend.

## Testing Pyramid

```
       /\
      /  \     E2E Tests (10%)
     /____\    
    /      \   Integration Tests (30%)
   /________\ 
  /          \ Unit Tests (60%)
 /____________\
```

---

## 1. Unit Tests

### Coverage Targets
| Layer | Target Coverage |
|-------|-----------------|
| Services | 85% |
| Controllers | 70% |
| Models | 90% |
| Utils | 95% |

### Example: Ride Service Unit Test
```go
package services_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

func TestRideService_CreateRide(t *testing.T) {
    // Arrange
    mockRepo := new(mock.RideRepository)
    mockPricing := new(mock.PricingService)
    service := NewRideService(mockRepo, mockPricing)
    
    req := CreateRideRequest{
        Pickup: Location{Lat: 19.0760, Lng: 72.8777},
        Drop:   Location{Lat: 19.2183, Lng: 72.9781},
        VehicleType: "bike",
    }
    
    mockPricing.On("CalculateFare", req).Return(150.0, nil)
    mockRepo.On("Create", mock.Anything).Return(&Ride{ID: "ride-123"}, nil)
    
    // Act
    ride, err := service.CreateRide(req)
    
    // Assert
    assert.NoError(t, err)
    assert.Equal(t, "ride-123", ride.ID)
    assert.Equal(t, 150.0, ride.Fare)
    mockPricing.AssertExpectations(t)
    mockRepo.AssertExpectations(t)
}

func TestRideService_InvalidLocation(t *testing.T) {
    service := NewRideService(nil, nil)
    
    req := CreateRideRequest{
        Pickup: Location{Lat: 999, Lng: 999}, // Invalid
    }
    
    _, err := service.CreateRide(req)
    
    assert.Error(t, err)
    assert.Equal(t, ErrInvalidLocation, err)
}
```

### Running Unit Tests
```bash
# Run all unit tests
go test ./... -short

# Run with coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out

# Run specific package
go test ./services/... -v
```

---

## 2. Integration Tests

### Database Integration
```go
func TestRideRepository_Create(t *testing.T) {
    // Setup test database
    db := setupTestDB()
    defer teardownTestDB(db)
    
    repo := NewRideRepository(db)
    
    ride := &Ride{
        Pickup: Location{Lat: 19.0760, Lng: 72.8777},
        Drop:   Location{Lat: 19.2183, Lng: 72.9781},
        Status: "requested",
    }
    
    // Test
    created, err := repo.Create(ride)
    
    // Assert
    assert.NoError(t, err)
    assert.NotEmpty(t, created.ID)
    assert.Equal(t, "requested", created.Status)
    
    // Verify in database
    var found Ride
    db.First(&found, "id = ?", created.ID)
    assert.Equal(t, created.ID, found.ID)
}
```

### Redis Integration
```go
func TestDriverLocationService_Update(t *testing.T) {
    redis := setupTestRedis()
    defer teardownTestRedis(redis)
    
    service := NewDriverLocationService(redis)
    
    // Test location update
    err := service.UpdateLocation("driver-123", 19.0760, 72.8777)
    
    assert.NoError(t, err)
    
    // Verify in Redis
    loc, err := service.GetLocation("driver-123")
    assert.NoError(t, err)
    assert.Equal(t, 19.0760, loc.Lat)
}
```

### API Integration (HTTP)
```go
func TestRideAPI_CreateRide(t *testing.T) {
    router := setupTestRouter()
    
    payload := `{
        "pickup": {"lat": 19.0760, "lng": 72.8777, "address": "Mumbai"},
        "drop": {"lat": 19.2183, "lng": 72.9781, "address": "Thane"},
        "vehicle_type": "bike"
    }`
    
    req := httptest.NewRequest("POST", "/api/v1/rides", strings.NewReader(payload))
    req.Header.Set("Authorization", "Bearer "+getTestToken())
    req.Header.Set("Content-Type", "application/json")
    
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)
    
    assert.Equal(t, 201, w.Code)
    
    var response map[string]interface{}
    json.Unmarshal(w.Body.Bytes(), &response)
    
    assert.True(t, response["success"].(bool))
    assert.NotNil(t, response["data"].(map[string]interface{})["id"])
}
```

---

## 3. End-to-End Tests

### Test Scenarios

#### Happy Path: Complete Ride Flow
```go
func TestE2E_CompleteRide(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping E2E test in short mode")
    }
    
    // 1. Rider requests ride
    ride := requestRide(t, riderToken, pickup, drop)
    
    // 2. Driver accepts
    acceptRide(t, driverToken, ride.ID)
    
    // 3. Driver arrives
    driverArrives(t, driverToken, ride.ID)
    
    // 4. Ride starts
    startRide(t, driverToken, ride.ID, otp)
    
    // 5. Driver updates location during ride
    updateLocation(t, driverToken, 19.1000, 72.9000)
    
    // 6. Ride completes
    completeRide(t, driverToken, ride.ID)
    
    // 7. Payment processed
    processPayment(t, riderToken, ride.ID)
    
    // 8. Verify ride status
    rideStatus := getRideStatus(t, ride.ID)
    assert.Equal(t, "completed", rideStatus)
}
```

#### Error Scenarios
```go
func TestE2E_RideCancellation(t *testing.T) {
    // 1. Request ride
    ride := requestRide(t, riderToken, pickup, drop)
    
    // 2. Cancel before acceptance
    cancelRide(t, riderToken, ride.ID, "changed_mind")
    
    // 3. Verify no cancellation fee (within 2 min)
    fees := getCancellationFees(t, ride.ID)
    assert.Equal(t, 0.0, fees)
}

func TestE2E_DriverTimeout(t *testing.T) {
    // 1. Request ride
    ride := requestRide(t, riderToken, pickup, drop)
    
    // 2. Wait for timeout (no driver accepts)
    time.Sleep(2 * time.Minute)
    
    // 3. Verify auto-reassignment
    status := getRideStatus(t, ride.ID)
    assert.Equal(t, "searching", status)
}
```

---

## 4. Load Testing

### Tools
- **k6**: Scriptable load testing
- **Grafana**: Visualization
- **Prometheus**: Metrics

### Load Test Scenarios

#### Scenario 1: Peak Hour Simulation
```javascript
// loadtest/peak_hour.js
import http from 'k6/http';
import { check } from 'k6';

export const options = {
    stages: [
        { duration: '2m', target: 100 },   // Ramp up
        { duration: '5m', target: 100 },   // Steady state
        { duration: '2m', target: 200 },   // Peak
        { duration: '5m', target: 200 },   // Peak sustained
        { duration: '2m', target: 0 },     // Ramp down
    ],
    thresholds: {
        http_req_duration: ['p(95)<200'], // 95% under 200ms
        http_req_failed: ['rate<0.01'],     // <1% errors
    },
};

export default function() {
    const res = http.post('http://api.rapido.com/api/v1/rides', {
        pickup: { lat: 19.0760, lng: 72.8777 },
        drop: { lat: 19.2183, lng: 72.9781 },
        vehicle_type: 'bike',
    });
    
    check(res, {
        'status is 201': (r) => r.status === 201,
        'response time < 200ms': (r) => r.timings.duration < 200,
    });
}
```

#### Scenario 2: Driver Location Updates
```javascript
// loadtest/driver_location.js
export const options = {
    stages: [
        { duration: '1m', target: 1000 },  // 1000 drivers updating
        { duration: '5m', target: 1000 },
    ],
};

export default function() {
    const driverID = `driver-${__VU}`;
    const res = http.post('http://api.rapido.com/api/v1/drivers/location', {
        driver_id: driverID,
        lat: 19.0760 + Math.random() * 0.1,
        lng: 72.8777 + Math.random() * 0.1,
    });
    
    check(res, {
        'location updated': (r) => r.status === 200,
    });
}
```

### Load Test Targets

| Endpoint | RPS Target | P95 Latency | Error Rate |
|----------|-----------|-------------|------------|
| POST /rides | 500 | <200ms | <0.1% |
| GET /rides/:id | 1000 | <50ms | <0.1% |
| POST /drivers/location | 2000 | <100ms | <1% |
| POST /payments | 100 | <500ms | <0.01% |

---

## 5. Chaos Testing

### Failure Injection
```go
func TestChaos_RedisFailure(t *testing.T) {
    // Simulate Redis down
    stopRedis()
    defer startRedis()
    
    // System should fallback to database
    ride, err := rideService.CreateRide(req)
    
    assert.NoError(t, err) // Should still work
    assert.Equal(t, "requested", ride.Status)
}

func TestChaos_PaymentGatewayTimeout(t *testing.T) {
    // Simulate slow payment gateway
    mockPaymentGateway.Delay = 10 * time.Second
    
    // Should timeout and queue for retry
    err := paymentService.Process(payment)
    
    assert.Equal(t, ErrPaymentTimeout, err)
    
    // Verify queued for retry
    queued := paymentService.GetRetryQueue()
    assert.Contains(t, queued, payment.ID)
}
```

---

## 6. Security Testing

### Authentication
```go
func TestSecurity_InvalidToken(t *testing.T) {
    req := httptest.NewRequest("POST", "/api/v1/rides", nil)
    req.Header.Set("Authorization", "Bearer invalid_token")
    
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)
    
    assert.Equal(t, 401, w.Code)
}

func TestSecurity_ExpiredToken(t *testing.T) {
    expiredToken := generateExpiredToken()
    
    req := httptest.NewRequest("GET", "/api/v1/rides/123", nil)
    req.Header.Set("Authorization", "Bearer "+expiredToken)
    
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)
    
    assert.Equal(t, 401, w.Code)
}
```

### Rate Limiting
```go
func TestSecurity_RateLimit(t *testing.T) {
    // Send 101 requests (limit is 100/min)
    for i := 0; i < 101; i++ {
        req := httptest.NewRequest("POST", "/api/v1/auth/otp", nil)
        w := httptest.NewRecorder()
        router.ServeHTTP(w, req)
        
        if i < 100 {
            assert.NotEqual(t, 429, w.Code)
        } else {
            assert.Equal(t, 429, w.Code) // Rate limited
        }
    }
}
```

### SQL Injection
```go
func TestSecurity_SQLInjection(t *testing.T) {
    maliciousInput := "'; DROP TABLE users; --"
    
    req := httptest.NewRequest("GET", "/api/v1/rides?id="+maliciousInput, nil)
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)
    
    // Should return validation error, not 500
    assert.NotEqual(t, 500, w.Code)
}
```

---

## 7. Test Data Management

### Fixtures
```go
// test/fixtures/rides.go
var TestRides = []Ride{
    {
        ID:     "ride-001",
        Status: "completed",
        Fare:   150.0,
        Pickup: Location{Lat: 19.0760, Lng: 72.8777},
        Drop:   Location{Lat: 19.2183, Lng: 72.9781},
    },
    {
        ID:     "ride-002",
        Status: "cancelled",
        Fare:   0,
        Pickup: Location{Lat: 19.0760, Lng: 72.8777},
    },
}

// test/fixtures/drivers.go
var TestDrivers = []Driver{
    {
        ID:       "driver-001",
        Name:     "Test Driver",
        Rating:   4.8,
        Status:   "online",
        Location: Location{Lat: 19.0760, Lng: 72.8777},
    },
}
```

### Test Database Seeding
```go
func seedTestDatabase(db *gorm.DB) {
    db.Create(&TestDrivers)
    db.Create(&TestRides)
}
```

---

## 8. CI/CD Integration

### GitHub Actions Workflow
```yaml
# .github/workflows/test.yml
name: Test

on: [push, pull_request]

jobs:
  unit-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Run Unit Tests
        run: go test ./... -short -coverprofile=coverage.out
      
      - name: Upload Coverage
        uses: codecov/codecov-action@v3
        with:
          file: ./coverage.out

  integration-tests:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:15
        env:
          POSTGRES_PASSWORD: test
          POSTGRES_DB: rapido_test
      redis:
        image: redis:7
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
      
      - name: Run Integration Tests
        run: go test ./... -run Integration -v

  load-tests:
    runs-on: ubuntu-latest
    if: github.ref == 'refs/heads/main'
    steps:
      - uses: actions/checkout@v3
      
      - name: Run k6 Load Tests
        uses: grafana/k6-action@v0.3.1
        with:
          filename: loadtest/peak_hour.js
```

---

## 9. Test Metrics

### Coverage Dashboard
| Metric | Current | Target |
|--------|---------|--------|
| Overall Coverage | 82% | 85% |
| Unit Test Pass Rate | 100% | 100% |
| Integration Pass Rate | 98% | 100% |
| E2E Pass Rate | 95% | 100% |
| Flaky Tests | 2 | 0 |

### Performance Benchmarks
| Operation | Mean | P95 | P99 |
|-----------|------|-----|-----|
| Create Ride | 45ms | 80ms | 150ms |
| Accept Ride | 25ms | 50ms | 100ms |
| Location Update | 10ms | 20ms | 50ms |
| Fare Calculation | 5ms | 10ms | 20ms |

---

## 10. Testing Checklist

### Before Release
- [ ] All unit tests passing
- [ ] Integration tests passing
- [ ] E2E critical path tests passing
- [ ] Load tests meeting SLA
- [ ] Security tests passing
- [ ] Code coverage >= 85%
- [ ] No flaky tests
- [ ] Performance regression < 10%
- [ ] Chaos tests passing

### Commands Summary
```bash
# Quick check
go test ./... -short

# Full suite
go test ./... -v -cover

# Integration only
go test ./... -run Integration -v

# Load test
k6 run loadtest/peak_hour.js

# Coverage report
go test ./... -coverprofile=coverage.out && go tool cover -html=coverage.out
```
