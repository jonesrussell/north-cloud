# Social Publisher Backend Additions

**Status:** Approved
**Date:** 2026-03-01
**Context:** Backend changes needed to support a complete frontend for the social-publisher service.

---

## 1. Content List Endpoint

**Route:** `GET /api/v1/content`

**Query Parameters:**
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `offset` | int | 0 | Pagination offset |
| `limit` | int | 50 | Page size (max 100) |
| `status` | string | — | Filter: `pending`, `delivered`, `failed` |
| `type` | string | — | Filter: `blog_post`, `social_update`, `product_announcement` |

**Response (200):**
```json
{
  "items": [
    {
      "id": "uuid",
      "type": "blog_post",
      "title": "My Article",
      "summary": "Short summary",
      "url": "https://example.com",
      "project": "personal",
      "source": "api",
      "published": false,
      "scheduled_at": "2026-03-15T10:00:00Z",
      "created_at": "2026-03-01T10:00:00Z",
      "delivery_summary": {
        "total": 2,
        "pending": 0,
        "delivered": 1,
        "failed": 1,
        "retrying": 0
      }
    }
  ],
  "count": 1,
  "total": 42,
  "offset": 0,
  "limit": 50
}
```

**Implementation:**
- New `ListContent` handler in `handler.go`
- New `ListContent` and `CountContent` repo methods
- SQL joins content with delivery status aggregates via subquery
- Status filter maps to delivery aggregate: `pending` = unpublished with no deliveries or all pending, `delivered` = at least one delivered, `failed` = all deliveries failed
- Body/images/tags/metadata omitted from list response (only in detail view via existing status endpoint)

---

## 2. Accounts CRUD

### 2a. List Accounts

**Route:** `GET /api/v1/accounts`

**Response (200):**
```json
{
  "items": [
    {
      "id": "uuid",
      "name": "personal-x",
      "platform": "x",
      "project": "personal",
      "enabled": true,
      "credentials_configured": true,
      "token_expiry": "2026-06-01T00:00:00Z",
      "created_at": "2026-03-01T10:00:00Z",
      "updated_at": "2026-03-01T10:00:00Z"
    }
  ],
  "count": 1
}
```

### 2b. Get Account

**Route:** `GET /api/v1/accounts/:id`

Same shape as list item. Returns 404 if not found.

### 2c. Create Account

**Route:** `POST /api/v1/accounts`

**Request:**
```json
{
  "name": "personal-x",
  "platform": "x",
  "project": "personal",
  "enabled": true,
  "credentials": {
    "api_key": "...",
    "api_secret": "...",
    "access_token": "...",
    "access_secret": "..."
  },
  "token_expiry": "2026-06-01T00:00:00Z"
}
```

- `name` and `platform` required
- `credentials` JSON is encrypted to BYTEA via AES-256-GCM before storage
- Encryption key from `SOCIAL_PUBLISHER_ENCRYPTION_KEY` env var
- Returns 201 with created account (credentials masked)

### 2d. Update Account

**Route:** `PUT /api/v1/accounts/:id`

Partial update — only provided fields are changed. If `credentials` is provided, re-encrypts.

### 2e. Delete Account

**Route:** `DELETE /api/v1/accounts/:id`

Hard delete. Returns 204 on success, 404 if not found.

### Encryption

- AES-256-GCM with random nonce per encryption
- Key: 32-byte hex string from `SOCIAL_PUBLISHER_ENCRYPTION_KEY`
- New `internal/crypto/` package with `Encrypt(plaintext, key)` and `Decrypt(ciphertext, key)` functions
- Credentials stored as `nonce || ciphertext` in BYTEA column

---

## 3. JWT Auth Middleware

Wire `infragin.ProtectedGroup` for all `/api/v1` routes:

```go
v1 := infragin.ProtectedGroup(router, "/api/v1", cfg.Auth.JWTSecret)
```

Health endpoints remain public (handled by infrastructure automatically).

---

## 4. Fix `scheduled_at` Parsing

The `PublishRequest` struct has `ScheduledAt` as `string`. Change to parse RFC3339 into `*time.Time` and pass through to the `PublishMessage` domain model when creating content.

---

## 5. Implement Retry Endpoint

**Route:** `POST /api/v1/retry/:id`

- `:id` is a delivery ID (not content ID)
- Validates delivery exists and is in `failed` status
- Resets: `status = 'retrying'`, `next_retry_at = NOW()`, clears `error`
- Retry worker picks it up on next poll cycle
- Returns 200 with updated delivery record
- Returns 404 if delivery not found
- Returns 400 if delivery is not in `failed` status

---

## 6. Wire X Adapter

Register the X adapter in the adapters map in `main.go`. The adapter loads credentials from the accounts table at publish time (not at startup), so it works once accounts are configured.

---

## Database Changes

New migration `002_add_content_list_indexes.up.sql`:

```sql
-- Support content list filtering
CREATE INDEX idx_content_type ON content (type);
CREATE INDEX idx_content_created ON content (created_at DESC);
CREATE INDEX idx_content_source ON content (source);
```

No schema changes needed — existing tables support all operations.

---

## Config Changes

New env vars:
- `SOCIAL_PUBLISHER_ENCRYPTION_KEY` — 32-byte hex key for AES-256-GCM credential encryption
- `SOCIAL_PUBLISHER_JWT_SECRET` already exists in config (rename from `AUTH_JWT_SECRET` pattern or use existing)
