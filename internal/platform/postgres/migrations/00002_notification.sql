-- +goose Up
SET LOCAL lock_timeout = '5s';
CREATE SCHEMA notification;
CREATE TABLE notification.messages (
    id uuid PRIMARY KEY,
    scope text NOT NULL CHECK (scope IN ('platform','tenant')),
    tenant_id text NOT NULL,
    idempotency_key text NOT NULL CHECK (length(idempotency_key) BETWEEN 1 AND 128),
    request_hash text NOT NULL,
    recipient text NOT NULL,
    template text NOT NULL,
    data jsonb NOT NULL,
    expires_at timestamptz NOT NULL,
    status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','processing','accepted','failed','unknown','expired')),
    delivery_status text NOT NULL DEFAULT 'pending' CHECK (delivery_status IN ('pending','deferred','delivered','bounced','blocked','complained')),
    delivery_at bigint NOT NULL DEFAULT 0,
    delivery_rank integer NOT NULL DEFAULT 0,
    attempt_count integer NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    lease_until timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((scope = 'platform' AND tenant_id = '') OR (scope = 'tenant' AND length(tenant_id) BETWEEN 1 AND 128)),
    UNIQUE (scope, tenant_id, idempotency_key)
);
CREATE INDEX messages_due ON notification.messages (next_attempt_at, created_at) WHERE status = 'queued';
CREATE INDEX messages_leased ON notification.messages (lease_until) WHERE status = 'processing';
CREATE TABLE notification.attempts (
    id uuid PRIMARY KEY,
    notification_id uuid NOT NULL REFERENCES notification.messages(id),
    number integer NOT NULL,
    status text NOT NULL CHECK (status IN ('processing','accepted','retry','failed','unknown')),
    message_id text UNIQUE,
    error_code text,
    created_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz,
    UNIQUE (notification_id, number)
);
-- Store only normalized delivery metadata, never raw webhook bodies or invitation URLs.
CREATE TABLE notification.events (
    event_key text PRIMARY KEY,
    message_id text NOT NULL,
    recipient text NOT NULL,
    kind text NOT NULL,
    occurred_at bigint NOT NULL,
    attempt_id uuid REFERENCES notification.attempts(id),
    received_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX events_unmatched ON notification.events (message_id) WHERE attempt_id IS NULL;

-- +goose Down
SET LOCAL lock_timeout = '5s';
DROP TABLE notification.events;
DROP TABLE notification.attempts;
DROP TABLE notification.messages;
DROP SCHEMA notification RESTRICT;
