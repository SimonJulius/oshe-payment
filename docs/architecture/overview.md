# OshePayment Architecture Overview

## 1. Introduction

OshePayment is a multi-tenant merchant acquiring and payment gateway platform designed to enable businesses to accept and manage digital payments.

The platform supports merchants that want to use OshePayment through a hosted back-office experience as well as merchants that want to integrate directly through APIs.

OshePayment is responsible for more than initiating payments. The platform is intended to manage the broader payment lifecycle, including:

- Merchant onboarding
- Payment initiation
- Hosted checkout
- Payment links
- QR payments
- Card-not-present payments
- Payment processing
- Transaction tracking
- Platform fees and commissions
- Merchant balances
- Ledger accounting
- Settlements
- Refunds
- Webhooks
- Reconciliation
- Developer integrations

Future versions of the platform may also support card-present payments through POS terminals and other payment channels.

---

## 2. Architectural Philosophy

OshePayment is initially implemented as a **modular monolith**.

The application runs as a single deployable backend system while maintaining clearly defined boundaries between major business domains.

The architecture is intentionally designed this way for the first stages of the project.

Starting directly with distributed microservices would introduce operational concerns such as:

- Network communication between services
- Service discovery
- Distributed tracing
- Cross-service authentication
- Distributed transactions
- Message delivery guarantees
- Infrastructure orchestration
- Deployment coordination
- Increased local development complexity

These concerns are important in large distributed systems, but they should be introduced only when the system has requirements that justify them.

The modular monolith allows OshePayment to establish strong domain boundaries while keeping development, debugging, testing, and deployment relatively simple.

Individual modules may later be extracted into independently deployable services when there is a clear reason to do so.

Examples of such reasons include:

- Independent scaling requirements
- Different availability requirements
- Independent deployment requirements
- Increased operational isolation
- Different security requirements
- Different ownership boundaries
- Performance bottlenecks
- Reliability requirements

The architecture therefore follows the principle:

> Start simple, establish strong boundaries, and distribute the system when the domain or operational requirements justify it.

---

## 3. High-Level System Context

At a high level, OshePayment sits between merchants, their customers, and the external financial infrastructure responsible for processing payments.

```text
┌──────────────────────┐
│      Merchant        │
│                      │
│ Dashboard / API      │
└──────────┬───────────┘
           │
           ▼
┌─────────────────────────────┐
│        OshePayment          │
│                             │
│ Merchant Acquiring          │
│ Payment Gateway             │
│ Payment Orchestration       │
│ Ledger                      │
│ Settlement                  │
└─────────────┬───────────────┘
              │
              ▼
┌─────────────────────────────┐
│ External Payment Processor  │
│ / Acquirer / Banking Layer  │
└─────────────┬───────────────┘
              │
              ▼
┌─────────────────────────────┐
│ Issuer / Bank / Card Scheme │
└─────────────────────────────┘
```

OshePayment will initially simulate some external financial infrastructure through internal test adapters.

Real integrations may be added later.

---

## 4. Tenancy Model

OshePayment is designed as a multi-tenant platform.

The platform operates at two primary business levels:

```text
OshePayment
    │
    ├── Merchant A
    │     ├── Users
    │     ├── Customers
    │     ├── Payments
    │     ├── Payment Links
    │     ├── API Keys
    │     ├── Webhooks
    │     ├── Balances
    │     └── Settlements
    │
    ├── Merchant B
    │     └── ...
    │
    └── Merchant N
          └── ...
```

OshePayment acts as the acquiring platform.

Merchants onboard onto OshePayment and use the platform to accept payments from their customers.

Each merchant must be logically isolated from every other merchant.

A merchant must never be able to access another merchant's:

- Payments
- Customers
- API credentials
- Settlements
- Balances
- Payment links
- Webhooks
- Users
- Configuration

Tenant isolation is therefore considered a core architectural requirement rather than an application-level convenience.

---

## 5. Primary Actors

### 5.1 OshePayment Platform Administrator

A platform administrator operates OshePayment itself.

Platform-level responsibilities may include:

- Managing merchants
- Monitoring transactions
- Managing merchant pricing
- Viewing platform revenue
- Reviewing settlements
- Managing processor integrations
- Monitoring operational health
- Reviewing reconciliation results
- Managing platform-level configuration

---

### 5.2 Merchant

A merchant is a business that uses OshePayment to accept payments.

A merchant may interact with OshePayment through:

- Merchant dashboard
- Hosted checkout
- Payment links
- QR payments
- APIs
- Webhooks
- Future POS integrations

A merchant may have multiple users with different permissions.

---

### 5.3 Merchant Customer

A customer is an individual or organization paying a merchant.

Customers typically interact with OshePayment through customer-facing payment experiences such as:

- Hosted checkout
- Payment links
- QR codes
- Future merchant-integrated checkout components

---

### 5.4 External Payment Processor

An external payment processor represents the infrastructure responsible for actually processing payment instructions.

OshePayment should not tightly couple its payment domain to a specific processor.

Instead, processor integrations should be implemented behind a common abstraction.

Conceptually:

```text
Payment Domain
      │
      ▼
Payment Processor Interface
      │
      ├── Fake Processor
      ├── Processor A
      ├── Processor B
      └── Future Processor
```

This allows payment orchestration to remain independent of any specific external provider.

---

## 6. Initial Domain Modules

The modular monolith will initially be divided into business-oriented modules.

These modules are logical boundaries inside the same deployable application.

### 6.1 Identity and Access

Responsible for authentication and authorization.

Potential responsibilities include:

- Platform users
- Merchant users
- Authentication
- Password management
- Sessions
- Roles
- Permissions
- Access control
- API authentication

---

### 6.2 Merchant

Responsible for merchant lifecycle and merchant configuration.

Potential responsibilities include:

- Merchant creation
- Merchant profile
- Business information
- Merchant status
- Merchant configuration
- Merchant users
- Settlement configuration
- Merchant pricing configuration

---

### 6.3 Payment

Responsible for the lifecycle of a payment.

The Payment module is one of the central domains of OshePayment.

Potential responsibilities include:

- Payment intent creation
- Payment confirmation
- Payment state transitions
- Payment method selection
- Payment processing
- Processor communication
- Payment status
- Payment references
- Idempotency
- Payment metadata

A payment should be treated as a stateful resource rather than as a single synchronous request.

Example lifecycle:

```text
REQUIRES_PAYMENT_METHOD
        │
        ▼
REQUIRES_CONFIRMATION
        │
        ▼
PROCESSING
        │
        ├───────────────┐
        ▼               ▼
    SUCCEEDED         FAILED
```

Additional states may be introduced as the domain evolves.

---

### 6.4 Checkout

Responsible for customer-facing hosted payment sessions.

Potential responsibilities include:

- Checkout session creation
- Checkout expiration
- Customer redirection
- Payment method presentation
- Successful checkout handling
- Cancelled checkout handling

Hosted checkout should ultimately create or interact with the Payment domain rather than implementing a separate payment lifecycle.

---

### 6.5 Payment Link

Responsible for reusable or single-use payment URLs created by merchants.

Conceptually:

```text
Payment Link
     │
     ▼
Checkout Session
     │
     ▼
Payment Intent
     │
     ▼
Payment Processing
```

Payment links therefore act as an entry point into the core payment flow.

---

### 6.6 QR Payment

Responsible for payments initiated through QR codes.

QR codes should represent or resolve to payment information rather than creating an entirely separate payment processing model.

Conceptually:

```text
QR Code
   │
   ▼
Payment / Checkout Context
   │
   ▼
Payment Intent
   │
   ▼
Payment Processing
```

Future versions may support both static and dynamic QR codes.

---

### 6.7 Ledger

Responsible for the financial accounting representation of money movements within OshePayment.

The ledger should be based on double-entry accounting principles.

The ledger is separate from payment processing.

A successful payment may cause financial entries to be recorded, but a payment record itself is not the accounting ledger.

For example:

```text
Customer pays:            ₦10,000
Platform commission:         ₦200
Merchant entitlement:      ₦9,800
```

Conceptually:

```text
Debit  Processor Clearing      ₦10,000

Credit Merchant Payable         ₦9,800
Credit Platform Revenue           ₦200
```

This enables OshePayment to derive balances from accounting entries rather than treating balances as arbitrary mutable numbers.

---

## 7. Balance

Merchant balances represent the financial position of a merchant within OshePayment.

Balance concepts may eventually include:

- Pending balance
- Available balance
- Settled balance
- Reserved balance

Balances should be derived from or reconciled against the ledger.

The ledger remains the authoritative financial record.

---

## 8. Settlement

The Settlement domain is responsible for paying merchants the funds owed to them.

A simplified settlement lifecycle may look like:

```text
Payment succeeds
      │
      ▼
Ledger entries posted
      │
      ▼
Merchant funds become available
      │
      ▼
Settlement window reached
      │
      ▼
Settlement created
      │
      ▼
Transfer initiated
      │
      ▼
Settlement completed
```

Settlement should be asynchronous.

Future settlement policies may include:

- T+0
- T+1
- T+2
- Manual settlement
- Merchant-specific schedules

---

## 9. Pricing and Commission

OshePayment generates revenue by charging merchants for payment processing and related services.

Pricing should eventually support merchant-specific configuration.

Examples include:

```text
Card Payments
1.5%
Maximum fee: ₦2,000

QR Payments
0.75%

International Cards
3.5%
```

A payment may therefore involve several financial amounts:

```text
Gross Payment Amount
        │
        ├── Processor Cost
        ├── OshePayment Fee
        ├── Tax
        │
        ▼
Merchant Net Amount
```

Pricing rules should eventually be modeled independently from the payment lifecycle.

---

## 10. Webhooks

Merchants integrating directly with OshePayment APIs require asynchronous notifications when important events occur.

OshePayment will therefore provide webhook functionality.

Example events may include:

```text
payment.created
payment.processing
payment.succeeded
payment.failed
payment.refunded

settlement.created
settlement.completed
settlement.failed
```

Webhook delivery should eventually support:

- Signed payloads
- Delivery attempts
- Retry policies
- Exponential backoff
- Delivery logs
- Manual replay
- Endpoint disabling
- Event history

Webhook consumers must not be assumed to process an event exactly once.

Webhook delivery should therefore be designed around idempotent consumption.

---

## 11. API Access

Merchants should be able to use OshePayment without using the merchant dashboard.

The platform should expose APIs that allow merchants to perform operations such as:

```text
Create payment
Retrieve payment
Create checkout
Create payment link
Retrieve transactions
Initiate refund
Retrieve settlement
Manage webhook endpoints
```

Merchant API integrations should use dedicated credentials.

Example:

```http
Authorization: Bearer sk_test_xxxxxxxxx
```

Production and test environments should eventually use separate credentials.

---

## 12. Idempotency

Payment-related write operations must support idempotency.

A merchant may retry a request due to:

- Network timeout
- Client retry
- Application crash
- Load balancer retry
- Unknown request outcome

Repeated requests using the same idempotency key should not create duplicate financial operations.

Example:

```http
POST /v1/payments

Idempotency-Key: 1d4d6c46-...
```

Multiple identical requests using the same key should resolve to the same logical operation.

Idempotency is considered a core payment reliability requirement.

---

## 13. Synchronous and Asynchronous Processing

Not every operation in OshePayment should happen synchronously within an HTTP request.

The system will distinguish between operations requiring immediate responses and operations that can occur asynchronously.

Example:

```text
Merchant Request
      │
      ▼
Create Payment
      │
      ▼
Process Payment
      │
      ▼
Return Payment Status
```

Additional actions may occur asynchronously:

```text
Payment Succeeded
      │
      ├── Ledger Posting
      ├── Webhook Delivery
      ├── Notification
      ├── Analytics
      └── Settlement Eligibility
```

The initial modular monolith may use internal application events.

A message broker may be introduced later when there is a clear requirement for durable asynchronous messaging or independent service execution.

---

## 14. Persistence

PostgreSQL will be used as the primary relational datastore.

The initial implementation may use one PostgreSQL instance while maintaining strong logical ownership of data between modules.

Modules should not freely depend on the internal database representation of other modules.

The long-term goal is to preserve enough ownership that modules can eventually be extracted without requiring every service to share the same database.

Potential future separation:

```text
Merchant Module
      │
      ▼
Merchant Data

Payment Module
      │
      ▼
Payment Data

Ledger Module
      │
      ▼
Ledger Data

Settlement Module
      │
      ▼
Settlement Data
```

Physical database separation is not required during the early modular-monolith phase.

---

## 15. Security Principles

Security is a foundational requirement because OshePayment represents financial infrastructure.

The platform should follow principles such as:

- Least privilege
- Strong tenant isolation
- Secure credential storage
- Short-lived or revocable credentials where appropriate
- Auditability
- Input validation
- Explicit authorization
- Secrets management
- Encryption in transit
- Sensitive-data minimization

OshePayment should not store raw card credentials such as:

- Full primary account numbers unless a future compliant implementation explicitly requires it
- CVV values

Card processing should eventually rely on tokenization or external processor-provided secure payment components.

---

## 16. Observability

As the platform grows, it should provide enough observability to understand both technical and financial behavior.

This will eventually include:

- Structured logging
- Metrics
- Distributed tracing
- Payment processing latency
- Payment success rate
- Processor failure rate
- Webhook delivery health
- Settlement failures
- Infrastructure health

Observability should become progressively more sophisticated as the architecture becomes distributed.

---

## 17. Control Plane and Transaction Plane

OshePayment conceptually contains two different categories of functionality.

### Control Plane

The control plane manages configuration and administration.

Examples:

- Merchant onboarding
- Users
- Roles
- API keys
- Pricing
- Settlement configuration
- Webhook configuration
- Developer settings

### Transaction Plane

The transaction plane processes financial activity.

Examples:

- Payment creation
- Payment confirmation
- Processor communication
- Ledger posting
- Refunds
- Balance updates
- Settlement processing

This distinction becomes important as the platform evolves.

For example, temporary unavailability of the merchant administration dashboard should not necessarily make payment processing unavailable.

---

## 18. Initial Deployment Model

The first version of OshePayment will use a deliberately simple deployment model.

```text
┌─────────────────────────────┐
│       OshePayment API       │
│      Modular Monolith       │
└──────────────┬──────────────┘
               │
               ▼
┌─────────────────────────────┐
│         PostgreSQL          │
└─────────────────────────────┘
```

The system may initially be containerized using Docker.

Additional infrastructure such as:

- Redis
- Message brokers
- Kubernetes
- Distributed tracing
- Service discovery

should only be introduced when there is a concrete use case requiring them.

---

## 19. Evolution Toward Microservices

The modular monolith is not intended to prevent future distribution.

Instead, it provides a controlled path toward it.

A module may become a candidate for extraction when it develops clearly different operational characteristics.

For example:

```text
OshePayment Modular Monolith

Merchant
Payment
Ledger
Settlement
Webhook
```

may eventually evolve into:

```text
API Gateway
     │
     ├── Merchant Service
     ├── Payment Service
     ├── Ledger Service
     ├── Settlement Service
     └── Webhook Service
```

The decision to extract a module should be documented through an Architecture Decision Record.

Microservice extraction should solve a demonstrated problem rather than serve as an architectural objective by itself.

---

## 20. Initial Scope

The first meaningful version of OshePayment should focus on a limited but complete payment lifecycle.

Initial capabilities should include:

- Merchant onboarding
- Merchant authentication
- API authentication
- Payment intent creation
- Payment confirmation
- Simulated payment processing
- Successful and failed payments
- Idempotency
- Double-entry ledger
- Merchant balance
- Platform commission
- Settlement
- Webhook delivery
- Basic merchant dashboard
- Basic platform administration
- Dockerized local development
- Automated testing

Features such as the following should be introduced later:

- Payment links
- QR payments
- Refunds
- Multiple processors
- Advanced reconciliation
- POS payments
- Terminal management
- Card-present payments
- Disputes
- Risk engine
- Multi-currency
- Payment routing
- Processor failover

---

## 21. Architectural Principles

The development of OshePayment should follow several core principles.

### Domain boundaries over technical layers

Modules should represent business capabilities rather than simply grouping code into generic controllers, services, and repositories.

### Financial correctness over convenience

Operations involving money should prioritize correctness, traceability, auditability, and idempotency.

### Explicit state transitions

Payments, settlements, refunds, and similar financial resources should use explicit lifecycle states.

### Ledger as financial source of truth

Financial balances should be supported by accounting entries rather than arbitrary mutable counters.

### Idempotency by design

Any operation that could create duplicate financial effects should consider retries and duplicate requests from the beginning.

### Asynchronous where appropriate

Operations that do not need to block the originating request should eventually be handled asynchronously.

### Security by design

Tenant isolation, authentication, authorization, secrets management, and sensitive-data minimization should be considered during implementation rather than added afterward.

### Complexity must be earned

Distributed infrastructure, abstractions, and architectural patterns should be introduced only when they solve a concrete problem.

---

## 22. Architecture Decision Records

Important architectural decisions are documented separately under:

```text
/docs/adr/
```

Architecture Decision Records capture:

- The context surrounding a decision
- The available tradeoffs
- The selected approach
- The consequences of the decision
- Circumstances that may cause the decision to change

This document describes the current overall architecture.

ADRs explain why specific architectural decisions were made.