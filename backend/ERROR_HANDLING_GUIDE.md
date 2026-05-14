# Error Response Sanitization Guide

## Overview

This document describes the production-safe error handling strategy implemented in the Rapido backend. All API errors must be sanitized to:

1. **Hide internal details**: Database errors, stack traces, configuration details
2. **Be actionable**: Help users understand what went wrong
3. **Be secure**: Never expose sensitive information (file paths, queries, secrets)
4. **Be consistent**: Use the `SanitizedErrorResponse()` helper from `utils`

---

## Error Response Format

### Success Response
```json
{
  "success": true,
  "message": "Operation completed",
  "data": { ... }
}
```

### Error Response (Production-Safe)
```json
{
  "success": false,
  "message": "Operation failed",
  "error": ""
}
```

**Note**: The `error` field is intentionally empty for sensitive operations. Use generic `message` for user guidance.

---

## Error Categories & Responses

### 1. Validation Errors (400 Bad Request)
**What to expose**: Specific field validation errors, format issues  
**What to hide**: Nothing (validation errors are public)

```go
// ✅ CORRECT: Show which field is invalid
ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", "phone number must be 10 digits"))

// ❌ WRONG: Don't expose validation details
ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error() /* e.g., "json: cannot unmarshal string into int" */))
```

### 2. Authentication/Authorization Errors (401/403)
**What to expose**: Generic "invalid credentials" or "unauthorized"  
**What to hide**: Which field was wrong, user existence, password requirements

```go
// ✅ CORRECT: Generic message
ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Invalid credentials", ""))

// ❌ WRONG: Leaks information
ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Invalid credentials", "password hash mismatch"))
ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Invalid credentials", "user not found in database"))
```

### 3. Database/System Errors (500 Internal Server Error)
**What to expose**: Generic "Operation failed" message  
**What to hide**: Database error details, connection strings, table names, query details

```go
// ✅ CORRECT: Use SanitizedErrorResponse for DB errors
if err := db.Create(&user).Error; err != nil {
    ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to create user", err.Error()))
    return
}
// SanitizedErrorResponse will convert DB error to generic message

// ❌ WRONG: Exposes database details
if err := db.Create(&user).Error; err != nil {
    ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to create user", err.Error()))
    // Error might be: "UNIQUE constraint failed: users.email"
    return
}
```

### 4. Duplicate Record Errors (409 Conflict)
**What to expose**: "This record already exists" or "Email is already registered"  
**What to hide**: Database constraint names, table structure

```go
// ✅ CORRECT: User-friendly message
if strings.Contains(err.Error(), "unique") {
    ctx.JSON(http.StatusConflict, utils.ErrorResponse("Email already registered", ""))
    return
}

// ❌ WRONG: Exposes constraint name
ctx.JSON(http.StatusConflict, utils.ErrorResponse("Email already registered", "UNIQUE constraint failed: users.email"))
```

### 5. Not Found Errors (404 Not Found)
**What to expose**: "Resource not found"  
**What to hide**: Query details, why it wasn't found (unless filtering is the issue)

```go
// ✅ CORRECT: Generic not found
if err := db.First(&ride, rideID).Error; err != nil {
    ctx.JSON(http.StatusNotFound, utils.ErrorResponse("Ride not found", ""))
    return
}

// ⚠️ CONDITIONAL: Can expose filtering context if relevant
if !user.IsActive {
    ctx.JSON(http.StatusNotFound, utils.ErrorResponse("Ride not found", "ride belongs to inactive user"))
    return
}
```

---

## Implementation Pattern

### Pattern 1: Database Operations
```go
// Database error → SanitizedErrorResponse
if err := c.Service.CreateUser(user); err != nil {
    ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Failed to create user", err.Error()))
    return
}
```

### Pattern 2: Business Logic Validation
```go
// Validation error → ErrorResponse with specific message
if user.Balance < request.Amount {
    ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Insufficient balance", ""))
    return
}
```

### Pattern 3: Auth/Secrets
```go
// Auth error → Generic message, no details
if token.Valid != true {
    ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Invalid token", ""))
    return
}
```

### Pattern 4: External Service Failures
```go
// External API error → SanitizedErrorResponse
if err := smsService.SendOTP(phone, code); err != nil {
    ctx.JSON(http.StatusTooManyRequests, utils.SanitizedErrorResponse("Failed to send OTP", err.Error()))
    return
}
```

---

## Common Mistakes to Avoid

### ❌ Exposing Database Errors
```go
// WRONG
ctx.JSON(http.StatusInternalServerError, 
    utils.ErrorResponse("Operation failed", err.Error()))
// Leaks: "column does not exist", "syntax error", table names
```

### ❌ Exposing File Paths
```go
// WRONG
ctx.JSON(http.StatusInternalServerError, 
    utils.ErrorResponse("Failed to read config", err.Error()))
// Leaks: "/etc/rapido/config.yaml: permission denied"
```

### ❌ Exposing Secret Values
```go
// WRONG
ctx.JSON(http.StatusUnauthorized, 
    utils.ErrorResponse("Invalid token", fmt.Sprintf("JWT secret mismatch: %s != %s", secret1, secret2)))
// Leaks: Secret values!
```

### ❌ Leaking User Existence
```go
// WRONG - Don't distinguish "user not found" vs "wrong password"
if user == nil {
    ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Invalid", "user not found"))
}
if !checkPassword(user.PasswordHash, password) {
    ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Invalid", "password incorrect"))
}
// An attacker can now enumerate valid usernames

// CORRECT - Same error for both
ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Invalid credentials", ""))
```

### ❌ Exposing Stack Traces
```go
// WRONG
ctx.JSON(http.StatusInternalServerError, 
    utils.ErrorResponse("Operation failed", fmt.Sprintf("%+v", err)))
// Leaks: Full stack trace with file paths and line numbers
```

---

## Sanitization Rules

The `SanitizedErrorResponse()` helper automatically converts known error patterns:

| Internal Error | Exposed Message |
|---|---|
| `unique constraint failed` | "This record already exists" |
| `foreign key constraint` | "Referenced record not found" |
| `database` (any) | "Data operation failed" |
| `connection` (any) | "Service temporarily unavailable" |
| `timeout` | "Request timeout" |
| `permission` | "You do not have permission for this action" |
| `already` | "This action has already been completed" |
| Other | (empty string) |

---

## Testing Checklist

Before merging error handling changes:

- [ ] No database error details in response
- [ ] No file paths in response
- [ ] No secret values in response
- [ ] No stack traces in response
- [ ] No table/column names in response
- [ ] Generic message for auth failures (not "user not found" vs "wrong password")
- [ ] User-friendly message for validation errors
- [ ] Consistent error response format across all endpoints
- [ ] Test with invalid requests (should not expose details)
- [ ] Test with real database errors (constraint violations, timeouts)

---

## Quick Migration

To sanitize an endpoint:

1. **Find** error responses with `.Error()`:
   ```bash
   grep -r "ErrorResponse.*err\.Error()" controllers/
   ```

2. **Categorize**:
   - Database operations → use `SanitizedErrorResponse()`
   - Validation/Auth → use `ErrorResponse()` with safe message
   - External APIs → use `SanitizedErrorResponse()`

3. **Replace**:
   ```go
   // Before
   utils.ErrorResponse("Failed to save", err.Error())
   
   // After (if it's a DB operation)
   utils.SanitizedErrorResponse("Failed to save", err.Error())
   ```

4. **Test** with invalid input to ensure no leakage

---

## Logging Internal Errors

**Important**: While **responses** must be sanitized, **logs** should include full details for debugging.

```go
import "go.uber.org/zap"

if err := db.Create(&user).Error; err != nil {
    // Log full error for debugging
    utils.Error("Failed to create user", zap.Error(err), zap.Any("user", user))
    
    // Return sanitized error to client
    ctx.JSON(http.StatusInternalServerError, 
        utils.SanitizedErrorResponse("Failed to create user", err.Error()))
    return
}
```

---

## Status: ✅ In Progress

This sanitization is being rolled out across high-impact endpoints:
- ✅ Auth controller (OTP, login, password)
- ✅ Utils helper (`SanitizedErrorResponse` added)
- 🔄 Payment controller (in progress)
- 🔄 Driver/Ride controllers (in progress)
- ⏳ Support and notification controllers (future)

All new code must follow these patterns. Update old code as you touch it.

---

**Last Updated**: May 14, 2026  
**Version**: 1.0
