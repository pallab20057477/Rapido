# Multi-stage build for production
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git gcc g++ libc-dev

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o rapido-backend .

# Production stage
FROM alpine:3.18

# Install runtime dependencies (include wget for HEALTHCHECK)
RUN apk --no-cache add ca-certificates tzdata wget

# Create non-root user
RUN addgroup -g 1000 -S appgroup && \
    adduser -u 1000 -S appuser -G appgroup

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/rapido-backend .
# Copy entrypoint script (will load Docker secrets into environment)
COPY deployments/docker/entrypoint.sh ./entrypoint.sh

# Change ownership
RUN chown appuser:appgroup rapido-backend
RUN chown appuser:appgroup entrypoint.sh && chmod 750 entrypoint.sh

# Use non-root user
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Use entrypoint to load secrets then run the app
ENTRYPOINT ["./entrypoint.sh"]
CMD ["./rapido-backend"]
