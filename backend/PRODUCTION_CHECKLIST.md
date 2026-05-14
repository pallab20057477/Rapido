# Production Readiness Checklist

## Status: ✅ PRODUCTION READY (with caveats)

This document describes the production readiness of the Rapido backend and any remaining work items.

---

## ✅ Completed Production Hardening

### Configuration & Secrets Management
- [x] Config validation (`config.Validate()`) rejects empty/placeholder values in production
- [x] Required secrets enforced at startup for `production` and `staging` environments
- [x] Redis is fatal in production (not optional)
- [x] SMS provider must be configured (Twilio or MSG91)
- [x] Razorpay webhook secret required
- [x] Admin credentials must be set (not placeholders)

### Authentication & Security
- [x] JWT token validation with expiry checks
- [x] Token blacklist for logout (stored in Redis)
- [x] OTP generation and storage in Redis with hashing (SHA-256)
- [x] OTP request fails in production if SMS delivery fails
- [x] Dev-mode OTP bypass (`DEV_TEST_OTP`) rejected in production
- [x] Query-parameter token auth restricted to development environments only
- [x] WebSocket origin validation with allowlist (no wildcard in auth middleware)
- [x] WebSocket empty-origin acceptance gated to development

### API Security
- [x] CORS origin allowlist (via `CORS_ALLOW_ORIGIN` env var)
- [x] Payment webhook route made public
- [x] Webhook signature validation (HMAC) when secret is configured
- [x] Missing webhook secret fails closed outside development
- [x] Empty webhook signatures rejected
- [x] Rate limiting on OTP attempts (5 failed attempts per 10 minutes)

### Logging & Observability
- [x] Structured logging via zap (not stdlib print/log in controllers)
- [x] Request ID tracking middleware
- [x] Audit logging middleware for sensitive actions
- [x] Production mode disables Gin debug console output
- [x] Debug prints replaced with structured logger calls

### Database & Persistence
- [x] GORM ORM with prepared statements (SQL injection safe)
- [x] Database migrations at startup
- [x] Indexes for performance (production-grade created)
- [x] Connection pooling via GORM defaults

### Graceful Shutdown
- [x] Signal handling (SIGINT, SIGTERM) with 30-second timeout
- [x] Worker pool graceful stop
- [x] Database connection closure

---

## ⚠️ Known Limitations & TODOs

### Enhancement TODOs (Non-Blocking)
These are feature gaps that don't affect core production operation:

1. **Bulk Admin Service** (`services/bulk_admin_service.go:316`)
   - TODO: Send welcome SMS to new drivers
   - Impact: Drivers won't receive SMS on bulk import (low priority for MVP)
   - Status: Nice-to-have, can add later

2. **Payment Reconciliation** (`services/payment_reconciliation.go:432`)
   - TODO: Implement remote gateway fetch and comparison
   - Impact: Pending payment reconciliation is marked as unmatched until compared
   - Status: Important for accounting, plan within Q2

3. **SIM Swap Detection** (`services/sim_swap_detection.go:124`)
   - TODO: Implement actual SIM swap fraud detection
   - Impact: Placeholder returns no fraud detected
   - Status: Enhancement for future security hardening

4. **Wallet Service** (`services/payment_outbox.go:202`)
   - TODO: Implement wallet event processing
   - Impact: Wallet refunds not yet implemented
   - Status: Planned for Phase 2 (in-app wallet credits)

5. **Delivery Status Tracking** (`services/sms_service.go:216`)
   - TODO: Implement SMS delivery confirmation via provider API
   - Impact: SMS delivery status not tracked
   - Status: Nice-to-have for monitoring

### Known Operational Concerns
1. **Database Connection**: Ensure PostgreSQL is accessible and initialized before startup
2. **Redis Connection**: Critical in production; startup will fail if unavailable
3. **SMS Provider**: At least one provider (Twilio/MSG91) must be configured
4. **Razorpay Webhook Secret**: Must match the secret configured in Razorpay dashboard
5. **CORS Allowlist**: Must match frontend origin(s); wildcard `*` is supported but not recommended

---

## 🚀 Pre-Deployment Steps

### 1. Environment Setup
```bash
# Set production environment
export APP_ENV=production

# Ensure all production secrets are configured
export DB_HOST=<real-hostname>
export DB_USERNAME=<real-user>
export DB_PASSWORD=<strong-password>
export JWT_SECRET=<32-char-random>
export JWT_REFRESH_SECRET=<32-char-random>
export ADMIN_EMAIL=<production-admin-email>
export ADMIN_PASSWORD=<strong-password>
export RAZORPAY_KEY_ID=<key>
export RAZORPAY_KEY_SECRET=<secret>
export RAZORPAY_WEBHOOK_SECRET=<secret>
export REDIS_ADDR=<hostname:port>
export TWILIO_ACCOUNT_SID=<or-MSG91_AUTH_KEY>
export TWILIO_AUTH_TOKEN=<or-MSG91_SENDER_ID>
export TWILIO_FROM_NUMBER=<or skip if using MSG91>
```

### 2. Data Initialization
```bash
# Migrations run automatically at startup; ensure database exists:
createdb rapido  # or equivalent for your RDBMS
```

### 3. Validation
```bash
# Run the production validation script
bash scripts/validate_production.sh

# Or manually verify:
go build ./...
go vet ./...
go test ./...
```

### 4. Startup Verification
```bash
# Start the backend; it will validate config and fail fast if secrets are missing
go run main.go

# Verify logs show:
# - Database connected
# - Migrations completed
# - Admin bootstrap completed
# - Redis connected
# - Workers started
# - Health routes initialized
# - Server listening on configured port
```

### 5. Health Check
```bash
curl http://localhost:8080/health
# Expected: {"status":"ok"}
```

### 6. Smoke Test (Optional)
- OTP request: `POST /api/v1/auth/otp/request` with valid phone
- OTP verify: `POST /api/v1/auth/otp/verify` with phone, email, OTP
- Ride request: `POST /api/v1/rides` while authenticated
- WebSocket: Connect to `GET /ws` with valid JWT header

---

## 📋 Configuration Reference

### Required (Production)
| Variable | Purpose | Example |
|----------|---------|---------|
| `APP_ENV` | Environment mode | `production` |
| `DB_HOST` | Database hostname | `postgres.prod.local` |
| `DB_USERNAME` | DB user | `rapido_app` |
| `DB_PASSWORD` | DB password | `(strong-password)` |
| `DB_DATABASE` | Database name | `rapido_prod` |
| `JWT_SECRET` | JWT signing key | `(32-char-random)` |
| `REDIS_ADDR` | Redis address | `redis.prod.local:6379` |
| `ADMIN_EMAIL` | Admin user email | `admin@rapido.com` |
| `ADMIN_PASSWORD` | Admin password | `(strong-password)` |

### Required if Using Payment Gateway
| Variable | Purpose |
|----------|---------|
| `RAZORPAY_KEY_ID` | Razorpay public key |
| `RAZORPAY_KEY_SECRET` | Razorpay secret key |
| `RAZORPAY_WEBHOOK_SECRET` | Webhook signature secret |

### Required if Using SMS
| Variable | Purpose |
|----------|---------|
| `TWILIO_ACCOUNT_SID` | Twilio account SID (for Twilio) |
| `TWILIO_AUTH_TOKEN` | Twilio auth token (for Twilio) |
| `TWILIO_FROM_NUMBER` | Twilio phone number (for Twilio) |
| **OR** | **OR** |
| `MSG91_AUTH_KEY` | MSG91 auth key (for MSG91 - India) |
| `MSG91_SENDER_ID` | MSG91 sender ID (default: `RAPIDO`) |

### Optional (Production Defaults Shown)
| Variable | Default | Purpose |
|----------|---------|---------|
| `SERVER_PORT` | `8080` | Server listen port |
| `GIN_MODE` | `release` | Gin web framework mode |
| `CORS_ALLOW_ORIGIN` | `""` (deny all) | CORS allowlist (comma-separated origins) |
| `OTP_EXPIRY_MINUTES` | `5` | OTP validity duration |
| `DB_SSLMODE` | `disable` | PostgreSQL SSL mode (set to `require` for prod) |

---

## 🔒 Security Checklist

- [ ] All secrets are stored securely (AWS Secrets Manager, HashiCorp Vault, etc.)
- [ ] Never commit `.env` or secrets to version control
- [ ] Use strong, random passwords for admin and database
- [ ] Enable TLS for database connections (`DB_SSLMODE=require`)
- [ ] Enable TLS for Redis connections (if available)
- [ ] Restrict CORS allowlist to frontend origin(s) only
- [ ] Enable rate limiting on auth endpoints (e.g., OTP requests)
- [ ] Monitor webhook signature validation logs
- [ ] Regularly rotate JWT secrets
- [ ] Use a secrets vault for production deployment
- [ ] Audit logs are enabled and shipped to a logging service

---

## 📝 Monitoring & Alerts

### Key Metrics to Monitor
1. **OTP Request Failure Rate**: Should be <1% (mostly valid requests)
2. **Failed JWT Validations**: Should be low (indicates token tampering or clocks out of sync)
3. **Webhook Signature Failures**: Should be zero (indicates misconfigured webhook secret)
4. **Redis Connection Errors**: Should be zero in production
5. **Database Connection Pool Exhaustion**: Monitor for connection leaks
6. **WebSocket Connection Count**: Per-user limits can be set if needed

### Recommended Alerts
- [ ] App fails to start (config validation error)
- [ ] Database connection lost
- [ ] Redis connection lost  
- [ ] Repeated webhook signature failures (misconfiguration)
- [ ] OTP request spike (potential attack)
- [ ] High rate of failed JWT validations

---

## 🚨 Troubleshooting

### App Won't Start: "Invalid production configuration"
- Check `APP_ENV` is not set to `production`/`staging`, OR
- Verify all required secrets are set (see above), OR
- Remove placeholder values like `CHANGE_ME` or `REPLACE_ME`

### OTP Not Sending
- Verify SMS provider credentials
- Check SMS provider account has credits/balance
- Review logs for provider API errors
- In production, OTP requests fail if SMS send fails (by design)

### WebSocket Connection Refused
- Verify client sends `Authorization: Bearer <token>` header
- For development only: Can use `?token=<token>` query param
- Check `CORS_ALLOW_ORIGIN` includes client origin

### Webhook Signature Invalid
- Verify webhook secret matches Razorpay/provider config
- Check webhook body is not modified before signature validation
- Ensure webhook secret is not a placeholder

---

## ✨ Version Info

- **Backend**: Rapido v1.0+
- **Go Version**: 1.20+
- **Database**: PostgreSQL 12+
- **Redis**: 6.0+
- **Validation Added**: May 2026

---

## 📞 Support

For production issues, refer to:
1. Application logs (structured via zap)
2. Server health endpoint: `GET /health`
3. Database and Redis connectivity checks
4. Webhook signature validation logs

---

**Last Updated**: May 14, 2026  
**Status**: ✅ Ready for production deployment with required secrets configured
