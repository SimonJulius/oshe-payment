# ADR-003: Use SQL and PostgreSQL for Authoritative Payment Data

## Status

Accepted

## Context

OshePayment must preserve relationships between merchants, payments, ledger
entries, fees, balances, and settlements. Retried requests and concurrent workers
must not create duplicate financial effects. The initial deployment is a Go
modular monolith operated by one engineer.

## Decision

Use PostgreSQL as the primary system of record and SQL migrations managed by
Goose. Use pgx for application queries and connection pooling. Keep module data
ownership explicit even while modules share a database.

### Why SQL over NoSQL

The core domain is relational: a payment belongs to a merchant, ledger entries
belong to a journal, and settlements refer to financial obligations. Foreign
keys, uniqueness, check constraints, and transactions let the database enforce
important invariants alongside application validation. SQL also supports joins
and aggregations for reconciliation and financial reporting without maintaining
extra copies of authoritative data.

Atomic transactions will let related financial writes commit or fail together.
Isolation and row locking provide explicit tools for concurrent changes. These
features do not automatically produce correct accounting: balanced journals,
idempotency, tenant authorization, transaction boundaries, and retry handling
still require deliberate domain implementation and tests.

NoSQL is a broad category, and some NoSQL databases support transactions and
strong consistency. It is not inherently unsuitable for payments. For this
project, relational constraints and reporting fit the planned model directly;
a document or key-value store would add modeling and operational choices without
a demonstrated benefit. Revisit specialized storage for caches, search, or
analytics if measured requirements justify it.

### Why PostgreSQL

- Transactions, MVCC, isolation levels, row locks, and constraints support
  concurrent payment and accounting workflows.
- Exact numeric types and BIGINT support precise monetary representations.
  Future payment amounts should use integer minor units with explicit currency;
  never floating point. Fee rounding rules must be explicit.
- Composite foreign keys and unique indexes can enforce merchant-scoped
  relationships and idempotency. Row-level security is an optional additional
  defense, not a substitute for application authorization.
- JSONB allows flexible processor metadata while financial relationships remain
  typed and constrained. Sensitive card data must not be stored in metadata.
- A mature Go driver, familiar SQL tooling, backups, point-in-time recovery, and
  managed hosting options reduce operational work for a small team.
- One database keeps local development and initial deployment manageable.

MySQL is also a valid relational choice. PostgreSQL is selected for its combined
constraint, indexing, JSONB, and concurrency capabilities and its fit with pgx;
this decision does not claim that it is universally faster than alternatives.

## Migration and reliability policy

The implementation provides embedded, ordered SQL migrations, Goose version
history, transaction-per-migration execution, bounded lock waits, and a PostgreSQL
session advisory lock to serialize cooperating deployers. Migrations run as a
separate deployment step, never implicitly on API startup. Applied files are
immutable: corrections require a new migration. Goose version history does not
provide checksum verification; review and source control enforce file immutability.

The first migration creates only the platform infrastructure schema. Merchant,
payment, idempotency, and ledger tables will be introduced with their respective
domain implementations, rather than prematurely defining financial invariants.

For live schema evolution, use expand–migrate–contract: add compatible structures,
deploy compatible code, backfill in bounded resumable batches, verify correctness,
switch reads, and remove old structures only after old code is retired. Keep
large data backfills separate from schema migrations. Operations such as
CREATE INDEX CONCURRENTLY require an explicitly reviewed nontransactional
migration and a recovery plan. Do not simply disable transactions for all files.

This policy is informed by [Stripe's published online migration approach](https://stripe.com/blog/online-migrations),
which emphasizes staged changes and verification. It is not a claim of Stripe
certification, identical internal infrastructure, or guaranteed zero downtime.

Production uses separate migration-owner and runtime roles. The runtime role
must not be a superuser or have schema creation rights; grant only needed schema
usage and table/sequence privileges as domain migrations are added. Local Compose
credentials are development-only. Production connections must use authenticated
TLS (`sslmode=verify-full` with the appropriate trusted CA), externally managed
secrets, and a tested backup/restore and recovery policy. Do not log connection
strings. Pool limits must be budgeted across replicas and workers.

## Consequences

Schema changes require review and deployment coordination. Transactions should
be short; external processor requests must not be performed while holding database
locks. Deadlocks and serialization failures require bounded retries of the whole
transaction, with application idempotency protecting financial effects.

PostgreSQL remains a single infrastructure dependency initially. Replication,
backup retention, recovery objectives, monitoring, and capacity planning must be
configured before production. Readiness currently checks connectivity; it does
not certify accounting correctness or deployment schema compatibility.

## References

- [PostgreSQL constraints](https://www.postgresql.org/docs/18/ddl-constraints.html)
- [PostgreSQL concurrency control](https://www.postgresql.org/docs/18/mvcc.html)
- [PostgreSQL numeric types](https://www.postgresql.org/docs/18/datatype-numeric.html)
- [Goose](https://github.com/pressly/goose)
- [Stripe online migrations](https://stripe.com/blog/online-migrations)

## Review criteria

Revisit when demonstrated query, availability, data-volume, or module-isolation
requirements exceed this design. Record new storage systems or service extraction
in a separate ADR.
