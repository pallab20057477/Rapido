# Environment Variables Checklist

This document helps you verify your `.env` file is properly configured for end-to-end operation.

## ✅ Required for Basic Operation (No external services)

| Variable | Example Value | Purpose | Status |
|----------|---------------|---------|--------|
| `SERVER_PORT` | `8080` | API server port | ⚠️ **CHECK** |
| `SERVER_HOST` | `0.0.0.0` | Bind address | ⚠️ **CHECK** |
| `GIN_MODE` | `debug` or `release` | Gin framework mode | ⚠️ **CHECK** |
| `DB_HOST` | `localhost` | PostgreSQL host | ⚠️ **CHECK** |
| `DB_PORT` | `5432` | PostgreSQL port | ⚠️ **CHECK** |
| `DB_DATABASE` | `rapido` | Database name | ⚠️ **CHECK** |
| `DB_USERNAME` | `postgres` | Database user | ⚠️ **CHECK** |
| `DB_PASSWORD` | `your_password` | Database password | ⚠️ **CHECK** |
| `DB_TIMEZONE` | `Asia/Kolkata` | Database timezone | ⚠️ **CHECK** |
| `REDIS_ADDR` | `localhost` or `localhost:6379` | Redis address | ⚠️ **CHECK** |
| `REDIS_PORT` | `6379` | Redis port | ⚠️ **CHECK** |
| `REDIS_PASSWORD` | (empty) or `password` | Redis password | ⚠️ **CHECK** |
| `REDIS_DB` | `0` | Redis database number | ⚠️ **CHECK** |
| `JWT_SECRET` | `min_32_characters_long_secret` | JWT signing key | ⚠️ **CHECK** |
| `JWT_ACCESS_EXPIRY_MINUTES` | `15` | Access token expiry | ⚠️ **CHECK** |
| `JWT_REFRESH_EXPIRY_DAYS` | `7` | Refresh token expiry | ⚠️ **CHECK** |
| `DEFAULT_CURRENCY` | `INR` | Default currency | ⚠️ **CHECK** |
| `PLATFORM_COMMISSION_PERCENT` | `20` | Platform fee % | ⚠️ **CHECK** |
| `DRIVER_SEARCH_RADIUS_KM` | `5` | Driver search radius | ⚠️ **CHECK** |
| `RIDE_REQUEST_TIMEOUT_SECONDS` | `45` | Ride matching timeout | ⚠️ **CHECK** |
| `OTP_EXPIRY_MINUTES` | `5` | OTP validity | ⚠️ **CHECK** |
| `PUBLIC_BASE_URL` | `http://localhost:8080` | Base URL | ⚠️ **CHECK** |
| `UPLOAD_BASE_PATH` | `./uploads` | File upload path | ⚠️ **CHECK** |

**Total Required: 23 variables**

---

## 📱 Optional: SMS/OTP (Twilio)

| Variable | Example Value | Purpose | Status |
|----------|---------------|---------|--------|
| `TWILIO_ACCOUNT_SID` | `ACxxxxxxxxxxxxxxxx` | Twilio Account SID | ⚠️ **OPTIONAL** |
| `TWILIO_AUTH_TOKEN` | `xxxxxxxxxxxxxxxx` | Twilio Auth Token | ⚠️ **OPTIONAL** |
| `TWILIO_FROM_NUMBER` | `+1234567890` | Twilio phone number | ⚠️ **OPTIONAL** |

**Note:** If empty, OTP will use mock mode (123456) for testing

---

## 🔐 Optional: Google OAuth Login

| Variable | Example Value | Purpose | Status |
|----------|---------------|---------|--------|
| `GOOGLE_CLIENT_ID` | `xxx.apps.googleusercontent.com` | Google OAuth Client ID | ⚠️ **OPTIONAL** |
| `GOOGLE_CLIENT_SECRET` | `xxxxxxxx` | Google OAuth Secret | ⚠️ **OPTIONAL** |

**Note:** If empty, Google login will be disabled

---

## 💳 Optional: Real Payments (Razorpay)

| Variable | Example Value | Purpose | Status |
|----------|---------------|---------|--------|
| `RAZORPAY_KEY_ID` | `rzp_test_xxxxx` | Razorpay Key ID | ⚠️ **OPTIONAL** |
| `RAZORPAY_KEY_SECRET` | `xxxxxxxx` | Razorpay Secret | ⚠️ **OPTIONAL** |

**Note:** If empty, payments will use mock mode

---

## 🔔 Optional: Push Notifications (FCM)

| Variable | Example Value | Purpose | Status |
|----------|---------------|---------|--------|
| `FCM_SERVER_KEY` | `AAAAxxxxx:APA91bxxxxx` | Firebase Cloud Messaging key | ⚠️ **OPTIONAL** |

**Note:** If empty, push notifications will be logged only

---

## 🏨 Optional: External CRM Integration

| Variable | Example Value | Purpose | Status |
|----------|---------------|---------|--------|
| `EXTERNAL_CRM_ENABLED` | `false` | Enable CRM integration | ⚠️ **OPTIONAL** |
| `EXTERNAL_CRM_BASE_URL` | `https://crm.example.com` | CRM API URL | ⚠️ **OPTIONAL** |
| `EXTERNAL_CRM_TOKEN` | `Bearer token` | CRM Auth token | ⚠️ **OPTIONAL** |
| `EXTERNAL_CRM_WEBHOOK_SECRET` | `secret` | Webhook validation | ⚠️ **OPTIONAL** |
| `EXTERNAL_CRM_WEBHOOK_API_KEY` | `api_key` | API key | ⚠️ **OPTIONAL** |
| `EXTERNAL_CRM_WEBHOOK_ALLOWED_IPS` | `1.2.3.4,5.6.7.8` | Allowed webhook IPs | ⚠️ **OPTIONAL** |
| `EXTERNAL_CRM_TIMEOUT_SECONDS` | `5` | API timeout | ⚠️ **OPTIONAL** |

**Note:** Used for hotel/voucher integrations

---

## 📁 Optional: AWS S3 File Uploads

| Variable | Example Value | Purpose | Status |
|----------|---------------|---------|--------|
| `AWS_ACCESS_KEY_ID` | `AKIAxxxxxx` | AWS Access Key | ⚠️ **OPTIONAL** |
| `AWS_SECRET_ACCESS_KEY` | `xxxxxxxx` | AWS Secret Key | ⚠️ **OPTIONAL** |
| `AWS_REGION` | `ap-south-1` | S3 Region | ⚠️ **OPTIONAL** |
| `AWS_BUCKET_NAME` | `rapido-uploads` | S3 Bucket | ⚠️ **OPTIONAL** |

**Note:** If empty, files saved to local `UPLOAD_BASE_PATH`

---

## 🌐 Optional: CORS

| Variable | Example Value | Purpose | Status |
|----------|---------------|---------|--------|
| `CORS_ALLOW_ORIGIN` | `http://localhost:3000` | Allowed origins | ⚠️ **OPTIONAL** |

---

## 🧪 Testing Configuration (Minimal Setup)

For local development/testing, you only need:

```env
# Server
SERVER_PORT=8080
SERVER_HOST=0.0.0.0
GIN_MODE=debug

# Database (PostgreSQL)
DB_HOST=localhost
DB_PORT=5432
DB_DATABASE=rapido
DB_USERNAME=postgres
DB_PASSWORD=your_password
DB_TIMEZONE=Asia/Kolkata

# Redis
REDIS_ADDR=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0

# JWT (generate strong secret for production)
JWT_SECRET=your_32_character_secret_key_here
JWT_ACCESS_EXPIRY_MINUTES=60
JWT_REFRESH_EXPIRY_DAYS=7

# App Settings
DEFAULT_CURRENCY=INR
PLATFORM_COMMISSION_PERCENT=20
DRIVER_SEARCH_RADIUS_KM=5
RIDE_REQUEST_TIMEOUT_SECONDS=45
OTP_EXPIRY_MINUTES=5
PUBLIC_BASE_URL=http://localhost:8080
UPLOAD_BASE_PATH=./uploads
```

**This minimal config (14 vars) lets you test all 70+ APIs without external services.**

---

## 🔒 Production Security Checklist

Before deploying to production, verify:

- [ ] `JWT_SECRET` is at least 32 characters, randomly generated
- [ ] `DB_PASSWORD` is strong and not default
- [ ] `REDIS_PASSWORD` is set (if Redis exposed)
- [ ] `GIN_MODE=release` (not debug)
- [ ] `RAZORPAY_KEY_ID` uses LIVE keys (not test)
- [ ] `TWILIO_*` uses real credentials
- [ ] `FCM_SERVER_KEY` is valid
- [ ] `AWS_*` has minimal IAM permissions
- [ ] HTTPS enabled (`PUBLIC_BASE_URL=https://...`)
- [ ] Database SSL enabled

---

## 🚀 Quick Validation Test

After setting up `.env`, run:

```bash
# 1. Test build
go build .

# 2. Test database connection
go run .  # Should start without DB errors

# 3. Test Redis connection
# Watch logs for "Redis connected"

# 4. Test first API
curl http://localhost:8080/health
# Expected: {"status":"healthy"}
```

---

## ❌ Common Missing Variables

These are often forgotten but break functionality:

1. **`JWT_SECRET`** - Must be set or app won't start
2. **`DB_TIMEZONE`** - Causes timestamp issues if wrong
3. **`PUBLIC_BASE_URL`** - Used in email/webhook URLs
4. **`UPLOAD_BASE_PATH`** - Must exist and be writable
5. **`REDIS_DB`** - Must be numeric (0, 1, 2...)

---

## 📊 Your .env Status

**Copy this checklist and mark ✅ for each variable you have:**

### Required Core (23 vars)
- [ ] SERVER_PORT
- [ ] SERVER_HOST
- [ ] GIN_MODE
- [ ] DB_HOST
- [ ] DB_PORT
- [ ] DB_DATABASE
- [ ] DB_USERNAME
- [ ] DB_PASSWORD
- [ ] DB_TIMEZONE
- [ ] REDIS_ADDR
- [ ] REDIS_PORT
- [ ] REDIS_PASSWORD
- [ ] REDIS_DB
- [ ] JWT_SECRET
- [ ] JWT_ACCESS_EXPIRY_MINUTES
- [ ] JWT_REFRESH_EXPIRY_DAYS
- [ ] DEFAULT_CURRENCY
- [ ] PLATFORM_COMMISSION_PERCENT
- [ ] DRIVER_SEARCH_RADIUS_KM
- [ ] RIDE_REQUEST_TIMEOUT_SECONDS
- [ ] OTP_EXPIRY_MINUTES
- [ ] PUBLIC_BASE_URL
- [ ] UPLOAD_BASE_PATH

### Optional Services
- [ ] TWILIO_ACCOUNT_SID (for real SMS)
- [ ] TWILIO_AUTH_TOKEN
- [ ] TWILIO_FROM_NUMBER
- [ ] GOOGLE_CLIENT_ID (for Google login)
- [ ] GOOGLE_CLIENT_SECRET
- [ ] RAZORPAY_KEY_ID (for real payments)
- [ ] RAZORPAY_KEY_SECRET
- [ ] FCM_SERVER_KEY (for push notifications)

---

**Compare your `.env` against `.env.example` to ensure nothing is missing.**
