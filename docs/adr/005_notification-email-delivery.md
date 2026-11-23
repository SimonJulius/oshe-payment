# ADR-005: Use a PostgreSQL Queue and Brevo for Email Notifications

## Status

Accepted

## Context

OshePayment is an invite-only, multitenant application. Auth will need to send
invitations, but Notification must remain independent of Auth and other business
modules, as established in [ADR-004](004_build-notification-first.md).

The notification architecture requires durable enqueueing, worker claims,
HTML and plain-text rendering, recorded attempts, provider acceptance, and
deduplicated delivery webhooks. Initial platform invitations may precede tenant
creation, while later notifications belong to existing tenants.

## Decision

Implement email delivery first, using Brevo's transactional email API, a
PostgreSQL-backed queue, and a separately runnable worker within the modular
monolith's codebase. Use local, versioned templates and one shared OshePayment
sender configured through environment variables.

Every request explicitly declares either platform scope or tenant scope.
Idempotency keys are unique within that scope and tenant; reusing a key for a
different payload is an error. Reads require the same scope. Platform scope is
not a wildcard for reading tenant notifications. Calling modules must authorize
scope and recipient before invoking Notification.

Auth owns invitation creation, token generation, expiry, revocation, and
redemption. It will persist its invitation and enqueue the notification in the
same PostgreSQL transaction through `EnqueueTx`. Notification accepts the
invitation URL and expiry without importing Auth or requiring its tables.

Workers atomically claim due messages with `FOR UPDATE SKIP LOCKED` and create an
attempt before contacting Brevo. Provider acceptance is separate from delivery
status. Rate limits receive bounded delayed retries; permanent rejection fails
the request. Transport errors, ambiguous responses, and abandoned worker leases
become `unknown` and are not automatically resent. Provider-side idempotency is
an extra defense, not an exactly-once delivery guarantee.

Authenticate webhooks with Brevo's configured bearer token. Correlate them to
stored attempts using provider message IDs or an opaque attempt ID in the
custom header, and verify the recipient. Persist normalized events, deduplicate
by message ID, recipient, event type, and event timestamp, and reconcile events
that arrive before acceptance is saved. Never use Brevo's webhook configuration
`id` as the event's unique identifier.

## Consequences

- Email requests survive restarts without introducing a message broker.
- Multiple worker processes can safely claim different requests.
- Templates and sender changes require controlled application configuration or
  deployment. Published template versions must remain stable for queued work.
- Delivery webhooks never invoke invitation creation or grant access.
- Unknown outcomes require operational reconciliation; automatic recovery favors
  avoiding duplicate invitations over resending an email of uncertain status.
- Invitation URLs and recipient addresses are sensitive queued data. Database
  access, backups, and retention must protect them; logs omit those values, and
  webhook storage excludes raw payloads.
- SMS, tenant-specific senders, Brevo-hosted templates, and Auth implementation
  are outside this version's scope.

## References

- [Brevo transactional email API](https://developers.brevo.com/reference/send-transac-email)
- [Brevo transactional webhooks](https://developers.brevo.com/docs/transactional-webhooks)
- [Brevo webhook authentication](https://developers.brevo.com/docs/secured-webhooks)
