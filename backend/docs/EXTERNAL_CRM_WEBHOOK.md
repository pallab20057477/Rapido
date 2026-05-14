# External CRM Webhook Contract

Rapido sends CRM events to the hotel CRM using a signed JSON envelope.

## Endpoint

`POST /webhooks/rapido`

## Headers

- `X-Rapido-Event-ID`: unique event identifier
- `X-Rapido-Event`: event name such as `user.upserted` or `ride.requested`
- `X-Rapido-Event-Version`: contract version, currently `1.0`
- `X-Rapido-Source`: sender name, currently `rapido-backend`
- `X-Rapido-Timestamp`: RFC3339Nano UTC timestamp
- `X-Rapido-Retry-Count`: retry attempt number
- `X-Rapido-Signature`: HMAC-SHA256 of `timestamp + "." + body`

## Body

```json
{
  "version": "1.0",
  "event_id": "uuid",
  "event": "user.upserted",
  "source": "rapido-backend",
  "entity_type": "user",
  "entity_id": "123",
  "occurred_at": "2026-05-01T12:00:00Z",
  "retry_count": 0,
  "data": {
    "name": "Asha",
    "phone": "9999999999"
  }
}
```

## Verification

The hotel CRM should verify the signature with the shared secret:

```text
signature = HMAC_SHA256(secret, timestamp + "." + body)
```

## Idempotency

- Use `event_id` as the idempotency key.
- Treat repeated events as success.
- Return `2xx` after a duplicate is recognized.

## Recommended response

```json
{
  "success": true,
  "message": "Webhook accepted",
  "data": {
    "event_id": "uuid",
    "event": "user.upserted",
    "entity_type": "user",
    "entity_id": "123",
    "duplicate": false
  }
}
```
