# Rapido Backend - Ride Sharing Application

A complete ride-sharing backend system built with Go, Gin, GORM, PostgreSQL, and Redis.

## Features

- **Authentication**: JWT-based auth with OTP verification (Twilio) and Google OAuth
- **User Management**: Riders, Drivers, and Admin roles
- **Driver Management**: Registration, verification, online/offline status, location tracking
- **Ride Booking**: Request rides, fare estimation, driver matching
- **Real-time Tracking**: WebSocket-based live location tracking
- **Payment Processing**: Wallet, UPI, Card, and Cash payments (Razorpay integration)
- **Admin Panel**: Dashboard, user management, driver verification, surge pricing
- **Safety Features**: SOS alerts, trip sharing, emergency contacts
- **Notifications**: Push notifications (FCM) and SMS

## Tech Stack

- **Language**: Go 1.21+
- **Web Framework**: Gin
- **ORM**: GORM
- **Database**: PostgreSQL
- **Cache**: Redis
- **WebSocket**: Gorilla WebSocket
- **Authentication**: JWT, OTP
- **Payment**: Razorpay

## Project Structure

```
.
├── config/          # Configuration management
├── controllers/     # HTTP request handlers
├── database/        # Database connection and migrations
├── middleware/      # Auth, CORS, rate limiting
├── models/          # Database models
├── routes/          # Route definitions
├── services/        # Business logic
├── utils/           # Utility functions
├── websocket/       # WebSocket handlers
├── .env             # Environment variables
├── .env.example     # Example environment file
├── .air.toml        # Air live reload config
├── go.mod           # Go dependencies
└── main.go          # Application entry point
```

## Quick Start

### Prerequisites

- Go 1.21 or higher
- PostgreSQL 14+
- Redis 6+
- Air (for live reload): `go install github.com/air-verse/air@latest`

### Installation

1. Clone the repository:
```bash
cd D:\GO\all\Rapido\backend
```

2. Copy environment file:
```bash
cp .env.example .env
# Edit .env with your configuration
```

3. Install dependencies:
```bash
go mod tidy
```

4. Run migrations and start server:
```bash
# Using Air (recommended for development)
air

# Or using Go directly
go run main.go
```

The server will start at `http://localhost:8080`

### API Documentation

#### Authentication
- `POST /api/v1/auth/otp/request` - Request OTP
- `POST /api/v1/auth/otp/verify` - Verify OTP and login
- `POST /api/v1/auth/refresh` - Refresh access token
- `POST /api/v1/auth/logout` - Logout
- `GET /api/v1/auth/profile` - Get user profile

#### Driver
- `POST /api/v1/driver/register` - Register as driver
- `POST /api/v1/driver/online` - Go online
- `POST /api/v1/driver/offline` - Go offline
- `POST /api/v1/driver/location` - Update location

#### Rides
- `POST /api/v1/rides` - Request a ride
- `GET /api/v1/rides/estimate` - Estimate fare
- `GET /api/v1/rides/active` - Get active ride
- `POST /api/v1/rides/:id/accept` - Accept ride (driver)
- `POST /api/v1/rides/:id/start` - Start ride
- `POST /api/v1/rides/:id/complete` - Complete ride

#### Payments
- `GET /api/v1/wallet` - Get wallet balance
- `POST /api/v1/wallet/add-money` - Add money to wallet
- `POST /api/v1/rides/:id/pay` - Process payment

#### Admin
- `GET /api/v1/admin/dashboard` - Dashboard stats
- `GET /api/v1/admin/drivers/pending` - Pending verifications
- `POST /api/v1/admin/drivers/verify` - Verify driver

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `SERVER_PORT` | Server port | 8080 |
| `DB_HOST` | Database host | localhost |
| `DB_PORT` | Database port | 5432 |
| `DB_USER` | Database user | postgres |
| `DB_PASSWORD` | Database password | - |
| `DB_NAME` | Database name | rapido_db |
| `JWT_SECRET` | JWT signing secret | - |
| `TWILIO_ACCOUNT_SID` | Twilio account SID | - |
| `TWILIO_AUTH_TOKEN` | Twilio auth token | - |
| `RAZORPAY_KEY_ID` | Razorpay key ID | - |
| `RAZORPAY_KEY_SECRET` | Razorpay key secret | - |

## Development

### Using Air for Live Reload

```bash
air
```

### Database Migrations

Migrations are handled automatically on startup. To manually run migrations:

```bash
go run main.go
```

### Testing

Run tests:
```bash
go test ./...
```

## Production Deployment

1. Set `GIN_MODE=release` in `.env`
2. Use a proper JWT secret
3. Configure production database and Redis
4. Set up SSL/TLS
5. Use a process manager (systemd, PM2, etc.)

## License

MIT
