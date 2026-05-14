package websocket

// DEPRECATED: Legacy handler file for backward compatibility.
// This file is kept for reference only. All WebSocket handling uses the unified
// HandleWebSocket method in handler.go. New clients should use:
// GET /ws with Authorization: Bearer <token> header (or Authorization header only).
//
// Query-parameter token auth (?token=xxx) is allowed ONLY in development mode
// to avoid credentials being exposed in server logs and browser history.
// Production deployments must use Authorization headers.
