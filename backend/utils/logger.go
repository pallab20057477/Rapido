package utils

import (
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.Logger
var Sugar *zap.SugaredLogger

// ANSI color codes
const (
	colorReset   = "\033[0m"
	colorRed     = "\033[91m"
	colorGreen   = "\033[92m"
	colorYellow  = "\033[93m"
	colorBlue    = "\033[94m"
	colorMagenta = "\033[95m"
	colorCyan    = "\033[96m"
	colorGray    = "\033[37m"
	colorBold    = "\033[1m"
)

// customColorLevelEncoder returns a custom colored level encoder
func customColorLevelEncoder(level zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	var color string
	switch level {
	case zapcore.DebugLevel:
		color = colorGray
	case zapcore.InfoLevel:
		color = colorGreen
	case zapcore.WarnLevel:
		color = colorYellow
	case zapcore.ErrorLevel:
		color = colorRed
	case zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel:
		color = colorRed + colorBold
	default:
		color = colorReset
	}
	enc.AppendString(color + level.CapitalString() + colorReset)
}

// customTimeEncoder returns a nicely formatted timestamp with color
func customTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	timeStr := t.Format("2006-01-02 15:04:05")
	enc.AppendString(colorCyan + timeStr + colorReset)
}

// customCallerEncoder returns a colored caller
func customCallerEncoder(caller zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(colorGray + caller.String() + colorReset)
}

// InitLogger initializes the structured logger with professional formatting
func InitLogger(env string) {
	var config zap.Config

	if env == "production" {
		config = zap.NewProductionConfig()
		config.EncoderConfig.TimeKey = "timestamp"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	} else {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = customColorLevelEncoder
		config.EncoderConfig.EncodeTime = customTimeEncoder
		config.EncoderConfig.EncodeDuration = zapcore.StringDurationEncoder
		config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
		config.OutputPaths = []string{"stdout"}
	}

	// Enhanced encoder config for better readability
	config.EncoderConfig.CallerKey = "caller"
	config.EncoderConfig.EncodeCaller = customCallerEncoder
	config.EncoderConfig.ConsoleSeparator = " │ "
	config.EncoderConfig.MessageKey = "message"
	config.EncoderConfig.LevelKey = "level"
	config.EncoderConfig.NameKey = "logger"

	var err error
	Logger, err = config.Build(zap.AddCaller(), zap.AddCallerSkip(1))
	if err != nil {
		panic(err)
	}

	Sugar = Logger.Sugar()
}

// Sync flushes the logger
func Sync() {
	if Logger != nil {
		Logger.Sync()
	}
}

// Debug logs debug message
func Debug(msg string, fields ...zap.Field) {
	Logger.Debug(msg, fields...)
}

// Info logs info message
func Info(msg string, fields ...zap.Field) {
	Logger.Info(msg, fields...)
}

// Warn logs warning message
func Warn(msg string, fields ...zap.Field) {
	Logger.Warn(msg, fields...)
}

// Error logs error message
func Error(msg string, fields ...zap.Field) {
	Logger.Error(msg, fields...)
}

// Fatal logs fatal message and exits
func Fatal(msg string, fields ...zap.Field) {
	Logger.Fatal(msg, fields...)
}

// With creates a child logger with fields
func With(fields ...zap.Field) *zap.Logger {
	return Logger.With(fields...)
}

// RequestLogFields creates standard request logging fields
func RequestLogFields(method, path, clientIP, userID string, status int, duration time.Duration, requestID string) []zap.Field {
	return []zap.Field{
		zap.String("method", method),
		zap.String("path", path),
		zap.String("client_ip", clientIP),
		zap.String("user_id", userID),
		zap.Int("status", status),
		zap.Duration("duration", duration),
		zap.String("request_id", requestID),
	}
}

// BusinessLogFields creates business event logging fields
func BusinessLogFields(event string, entityType string, entityID string, userID string) []zap.Field {
	return []zap.Field{
		zap.String("event_type", event),
		zap.String("entity_type", entityType),
		zap.String("entity_id", entityID),
		zap.String("user_id", userID),
		zap.Time("timestamp", time.Now()),
	}
}

// ErrorLogFields creates error logging fields
func ErrorLogFields(err error, errorType string, endpoint string) []zap.Field {
	fields := []zap.Field{
		zap.String("error_type", errorType),
		zap.String("endpoint", endpoint),
		zap.Time("timestamp", time.Now()),
	}
	if err != nil {
		fields = append(fields, zap.Error(err))
	}
	return fields
}

// PerformanceLogFields creates performance logging fields
func PerformanceLogFields(operation string, duration time.Duration, extraFields map[string]interface{}) []zap.Field {
	fields := []zap.Field{
		zap.String("operation", operation),
		zap.Duration("duration_ms", duration),
		zap.Time("timestamp", time.Now()),
	}
	for key, value := range extraFields {
		switch v := value.(type) {
		case string:
			fields = append(fields, zap.String(key, v))
		case int:
			fields = append(fields, zap.Int(key, v))
		case int64:
			fields = append(fields, zap.Int64(key, v))
		case float64:
			fields = append(fields, zap.Float64(key, v))
		case bool:
			fields = append(fields, zap.Bool(key, v))
		default:
			fields = append(fields, zap.Any(key, v))
		}
	}
	return fields
}

// GetEnv returns environment variable or default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// GinLogger returns a gin middleware for structured logging
func GinLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Log only when path is not being skipped
		duration := time.Since(start)
		status := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method
		requestID := c.GetString("requestID")
		userID := c.GetString("userID")

		if query != "" {
			path = path + "?" + query
		}

		fields := []zap.Field{
			zap.String("method", method),
			zap.String("path", path),
			zap.Int("status", status),
			zap.Duration("duration", duration),
			zap.String("client_ip", clientIP),
			zap.String("request_id", requestID),
			zap.String("user_id", userID),
		}

		// Log errors
		if len(c.Errors) > 0 {
			errorMsgs := make([]string, len(c.Errors))
			for i, err := range c.Errors {
				errorMsgs[i] = err.Error()
			}
			fields = append(fields, zap.Strings("errors", errorMsgs))
			Error("Request completed with errors", fields...)
			return
		}

		// Log based on status code
		switch {
		case status >= 500:
			Error("Server error", fields...)
		case status >= 400:
			Warn("Client error", fields...)
		default:
			Info("Request completed", fields...)
		}
	}
}
