# Durable signed webhooks

GoFormX webhooks are an optional part of the supported schema-first runtime. When enabled, accepting a submission and creating its delivery record occur in one PostgreSQL transaction. The event stores an encrypted snapshot of the destination configuration, so later endpoint changes cannot alter an already-accepted delivery.

## Enable the worker

Set WEBHOOK_ENABLED=true and provide a stable WEBHOOK_ENCRYPTION_KEY containing exactly 32 random bytes encoded as base64. Generate it once with openssl rand -base64 32, store it in the deployment vault, include it in backup/restore procedures, and do not rotate it until an explicit re-encryption tool exists. Losing the key makes queued encrypted configuration unrecoverable.

The API process owns a durable dispatcher loop. Pending records survive process or network failure. A stale processing lease is reclaimed after WEBHOOK_LOCK_TIMEOUT; delivery attempts stop at WEBHOOK_MAX_ATTEMPTS and enter dead_letter.

## Configure and observe

PUT /v1/forms/{formId}/webhook requires forms:write and accepts:

    {
      "url": "https://example.com/goformx",
      "headers": {"Authorization": "Bearer write-only-value"},
      "signingSecret": "at-least-32-characters-and-kept-by-the-receiver",
      "enabled": true
    }

The URL must use HTTPS port 443 and cannot contain credentials, a query, or a fragment. Every resolved address must be public both when the endpoint is configured and when a connection is opened. Proxies and redirects are disabled. Loopback, private, link-local, carrier-grade NAT, multicast, documentation, benchmarking, and reserved ranges are rejected.

The complete destination URL, headers, and signing secret are encrypted together with AES-256-GCM and bound to the form ID as authenticated data. Only the destination origin is returned or stored as plaintext; a potentially secret URL path never appears in API responses or delivery logs. Updating an endpoint requires supplying the complete desired secret configuration again.

Use GET /v1/forms/{formId}/deliveries?limit=25 with submissions:read to inspect recent status, attempt count, next attempt, HTTP status, and non-sensitive error category. Use POST /v1/forms/{formId}/deliveries/{deliveryId}/replay with forms:write to requeue a dead-letter delivery. Replay keeps the same delivery ID and event payload so receivers can remain idempotent.

## Verify a request

Each request includes:

- X-GoFormX-Delivery-ID: stable across every attempt and manual replay;
- X-GoFormX-Timestamp: Unix seconds for this attempt;
- X-GoFormX-Signature: v1= followed by the lowercase HMAC-SHA256 digest.

The signed bytes are:

    delivery_id + "." + timestamp + "." + exact_request_body

The receiver must read the raw body before JSON parsing, reject timestamps outside a five-minute window, calculate the HMAC using the delivery ID and timestamp headers exactly as shown above, compare signatures in constant time, and confirm the delivery ID matches the event ID in the body. Record the delivery ID before applying side effects. A repeated delivery ID should return a successful 2xx response without repeating the side effect.

Any 2xx response completes delivery. Network errors, 408, 429, and 5xx responses retry with capped exponential backoff. Other 4xx responses and configuration/authentication failures enter the dead letter immediately. Response bodies are discarded and never logged.
