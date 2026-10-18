# OshePayment

OshePayment is a multi-tenant merchant acquiring and payment gateway platform that enables businesses to accept digital payments through hosted checkout, payment links, QR codes, card-not-present transactions, and APIs.

The platform is designed to handle the broader payment lifecycle, including payment processing, platform fees and commissions, merchant balances, settlements, and other acquiring-related capabilities.

## Architecture

OshePayment is initially implemented as a modular monolith with clearly defined domain boundaries.

The architecture is designed so that selected modules can later be extracted into independently deployable services when justified by scaling, reliability, operational, or deployment requirements.

The project intentionally begins as a modular monolith to keep development and operational complexity manageable while the payment domain and service boundaries continue to evolve.

## Documentation

- [Architecture Overview](./docs/architecture/overview.md)
- [Architecture Decision Records](./docs/adr/)
- [SQL and PostgreSQL decision](./docs/adr/003_use-postgresql.md)
## Local development

Requires Go 1.26.2+, Docker Compose, and PostgreSQL 18 (provided by Compose).

```sh
cp .env.example .env
# Review .env before sourcing; this executes shell syntax.
set -a
. ./.env
set +a
docker compose up -d --wait postgres
go run ./cmd/migrate up
go run ./cmd/api
```

The API listens on `APP_PORT` (8080 by default). `GET /health/live` returns 204;
`GET /health/ready` returns 204 when PostgreSQL is reachable, otherwise 503.
SIGINT/SIGTERM gracefully stop the server and close the pool. The application
reads environment variables; it does not automatically load `.env`.

## Database migrations

```sh
go run ./cmd/migrate status
go run ./cmd/migrate up
# Disposable development databases only, after reviewing the SQL:
go run ./cmd/migrate -allow-down down
```

`status` reports the current applied version. `up` applies pending migrations;
`down` rolls back one migration. Files are embedded from
`internal/platform/postgres/migrations`; add the next numbered Goose SQL file
with `-- +goose Up` and `-- +goose Down` sections. Rebuild deployed binaries to
include new migrations. Do not edit migrations already applied elsewhere.

Run migrations once per deployment using a dedicated migration role before
starting compatible application code. Migrations are transactional by default
and cooperating runners serialize through an advisory lock. The command has a
two-minute deadline; schema SQL has a five-second lock timeout and thirty-second
statement timeout. Large backfills need separately designed resumable jobs.
Prefer corrective forward migrations in production; a down migration is not a
backup or an automatic deployment recovery strategy.

Compose uses development-only credentials and binds port 5432 to localhost.
PostgreSQL 18 stores data under a volume mounted at `/var/lib/postgresql`.
If you already have a volume created using the previous mount location, back it
up and inspect it before changing mounts; this change does not migrate old data.
Do not delete an existing volume to resolve a startup error.

Configuration: `DATABASE_URL` is required; `DATABASE_MAX_CONNS` defaults to 10,
`DATABASE_CONNECT_TIMEOUT` to `5s`, and `APP_PORT` to 8080. Production connections
must use `sslmode=verify-full`. See [ADR-003](docs/adr/003_use-postgresql.md) for
SQL/PostgreSQL rationale, role separation, and migration safety policy.

## Verification

```sh
go test ./...
go vet ./...
# WARNING: use an empty disposable database only; this creates and drops objects.
TEST_DATABASE_URL='postgres://user:password@localhost:5432/oshe_test?sslmode=disable' go test -race ./internal/platform/postgres -run TestIntegration -v
```

Integration tests are skipped unless `TEST_DATABASE_URL` is provided. They verify
connectivity, UTC sessions, repeated migrations, rollback protection and recovery,
and concurrent migration runners.
