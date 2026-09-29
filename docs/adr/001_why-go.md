# ADR-001: Use Go for the OshePayment Backend

## Status

Accepted

## Context

OshePayment is a merchant acquiring and payment gateway platform that will handle payment processing, merchant balances, settlements, webhooks, API integrations, and other financial operations.

The backend is expected to evolve from an initial modular monolith into a system where selected modules may eventually be extracted into independently deployable services.

The backend technology therefore needs to support:

- High-concurrency workloads
- Predictable performance
- Strong type safety
- Reliable network services
- Background and asynchronous processing
- Database-intensive workloads
- Clear service boundaries
- Straightforward deployment
- Long-term maintainability
- Future distributed-system requirements

The primary implementation options considered were:

- Go
- Node.js with TypeScript

Node.js is already familiar to the project's developer and would provide faster initial development.

However, one of the goals of OshePayment is not only to produce a working application but also to develop deeper backend and distributed-systems engineering experience.

Go provides an opportunity to learn these concepts in an ecosystem commonly used for network services, infrastructure, APIs, and highly concurrent backend systems.

## Decision

The primary backend language for OshePayment will be **Go**.

The initial application will be implemented as a modular monolith using Go and will progressively introduce more advanced infrastructure and distributed-system concepts as the requirements justify them.

Go was selected for both technical and learning reasons.

---

## Rationale

### 1. Concurrency Model

Payment systems perform many operations that involve waiting on external resources, including:

- Payment processors
- Banking APIs
- Databases
- Webhook endpoints
- Message brokers
- Internal services

Go provides lightweight concurrency through goroutines and communication primitives such as channels.

This provides a useful model for implementing concurrent and asynchronous workloads while keeping concurrency visible and explicit within the application.

The project can therefore explore areas such as:

- Concurrent processor requests
- Background workers
- Webhook delivery
- Settlement processing
- Reconciliation
- Concurrent request handling

Concurrency alone is not considered sufficient reason to choose Go, but it aligns well with the type of system OshePayment is intended to become.

---

### 2. Strong Static Typing

Financial systems benefit from explicit data models and compile-time verification.

Go's static type system helps make domain concepts explicit, including:

- Payments
- Money
- Merchants
- Ledger entries
- Settlements
- Payment statuses
- Processor responses

Static typing does not prevent business logic errors, but it can eliminate several classes of runtime errors and make contracts between different parts of the application clearer.

This becomes increasingly important as the application grows.

---

### 3. Explicit Error Handling

Go requires errors to be handled explicitly.

For example:

```go
result, err := processor.Process(ctx, payment)

if err != nil {
    return err
}
```

This style makes failure paths highly visible.

That is valuable in payment systems, where failure conditions are not exceptional edge cases but part of normal operation.

Examples include:

- Processor timeouts
- Declined payments
- Duplicate requests
- Database failures
- Webhook delivery failures
- Settlement failures
- Network interruptions

The project should treat error handling as part of the business flow rather than something hidden behind implicit exception propagation.

---

### 4. Predictable Runtime Characteristics

Go produces statically compiled binaries and has relatively low runtime overhead.

This provides benefits such as:

- Fast application startup
- Predictable deployment artifacts
- Relatively low memory overhead
- Good performance for network services
- Simple container images

Absolute performance is not currently a primary constraint for OshePayment.

However, predictable runtime characteristics are useful for infrastructure that may later process high transaction volumes.

---

### 5. Simple Deployment

A Go application can typically be compiled into a single executable binary.

This simplifies deployment compared with environments that require application source code and a language runtime to be installed together.

A production container can eventually resemble:

```text
Container
    │
    └── oshepayment binary
```

This is useful for:

- Docker deployments
- CI/CD pipelines
- Kubernetes deployments
- Horizontal scaling
- Reproducible builds

---

### 6. Suitable for Network Services

Go's standard library provides strong support for building HTTP and network applications.

Capabilities such as:

- `net/http`
- `context`
- JSON encoding and decoding
- Cryptography
- TLS
- Concurrency
- Testing

allow significant portions of the system to be built without depending on large application frameworks.

This provides an opportunity to understand the underlying mechanics of backend services rather than relying entirely on framework abstractions.

---

### 7. Context and Cancellation

Go's `context.Context` provides a standard mechanism for propagating:

- Request cancellation
- Deadlines
- Timeouts
- Request-scoped metadata

This is particularly relevant for payment systems.

For example:

```text
HTTP Request
     │
     ▼
Payment Service
     │
     ▼
Processor
     │
     ▼
Database
```

A cancelled request or expired deadline should be propagated through downstream operations where appropriate.

Learning to use context correctly is therefore an important part of the project's backend architecture.

---

### 8. Interface-Based Abstractions

Go interfaces can support clean boundaries between the OshePayment domain and external infrastructure.

For example:

```go
type PaymentProcessor interface {
    Authorize(ctx context.Context, payment Payment) (Result, error)
    Capture(ctx context.Context, payment Payment) (Result, error)
    Refund(ctx context.Context, payment Payment) (Result, error)
}
```

Implementations may eventually include:

```text
PaymentProcessor
      │
      ├── FakeProcessor
      ├── PaystackProcessor
      ├── FlutterwaveProcessor
      └── Other Processors
```

The payment domain should depend on the behavior required from a processor rather than a specific provider.

Go's small, implicit interfaces are well suited to this style of architecture.

---

### 9. Testing

Go includes testing support directly within its standard tooling.

The project intends to place significant emphasis on automated testing because financial operations require confidence in:

- State transitions
- Idempotency
- Ledger correctness
- Fee calculations
- Settlement calculations
- Authorization
- Tenant isolation

Go's standard testing tools provide a simple foundation that can be supplemented with integration and end-to-end testing where necessary.

---

### 10. Alignment With Future Architecture

OshePayment begins as a modular monolith, but selected modules may eventually become independent services.

Go is well suited to independently deployable backend services and infrastructure-oriented applications.

Potential future services include:

```text
Payment Service
Ledger Service
Settlement Service
Webhook Service
Reconciliation Service
Risk Service
```

Choosing Go at the beginning avoids introducing a language migration purely because the deployment model changes later.

This does not mean that all future OshePayment services must be written in Go.

The architecture should allow another language to be introduced where there is a strong technical reason.

---

## Alternatives Considered

### Node.js with TypeScript

Node.js was the strongest alternative.

It offers several advantages:

- Existing developer familiarity
- Faster initial implementation
- Large ecosystem
- Strong web development tooling
- Type safety through TypeScript
- Mature libraries for APIs, databases, queues, and testing

Using Node.js would be a valid technical choice for OshePayment.

It was not rejected because Node.js is incapable of supporting payment systems or high-scale backend applications.

The primary reason for selecting Go is that OshePayment is also intended to serve as a deliberate backend-engineering learning project.

Using Node.js would reduce the amount of unfamiliar backend material encountered because JavaScript and TypeScript are already established strengths of the developer.

Go therefore provides more opportunity to deepen knowledge of:

- Backend architecture
- Concurrency
- Memory and runtime behavior
- Network programming
- Explicit error handling
- Service design
- Distributed systems

### Other Languages

Languages such as Java, Kotlin, C#, and Rust could also support the requirements of OshePayment.

They were not selected because the project intends to focus its backend learning investment on Go rather than evaluate every technically viable backend language.

---

## Consequences

### Positive

Using Go provides:

- Strong static typing
- Explicit error handling
- Lightweight concurrency
- Good support for network services
- Simple compiled deployment artifacts
- Strong standard tooling
- Good alignment with future service extraction
- Significant backend learning opportunities

The choice also ensures that the project expands the developer's engineering capabilities rather than remaining primarily within an already familiar JavaScript ecosystem.

### Negative

The developer has significantly more professional experience with JavaScript and TypeScript than Go.

As a result:

- Initial development will be slower.
- Some basic implementation tasks will require additional research.
- Idiomatic Go practices will need to be learned progressively.
- Early architectural decisions may require refactoring as Go knowledge improves.
- Debugging unfamiliar language behavior may initially increase development time.

These costs are accepted because learning Go is an explicit objective of the project.

---

## Risks

### Overengineering While Learning Go

There is a risk of attempting to learn Go, payment architecture, microservices, messaging, DevOps, and distributed systems simultaneously.

To mitigate this, OshePayment will initially remain a modular monolith with limited infrastructure.

Complexity should be introduced incrementally.

### Writing JavaScript-Style Go

Existing JavaScript experience may influence Go code toward patterns that are not idiomatic in Go.

The codebase should therefore be periodically reviewed and refactored as understanding of Go conventions improves.

### Excessive Framework Dependence

Using a large framework too early could hide important aspects of Go's HTTP and application model.

The initial implementation should prefer the Go standard library and small, focused dependencies where practical.

---

## Learning Strategy

The project will follow a build-driven learning approach.

Instead of attempting to master Go completely before implementing OshePayment, concepts will be learned when they become necessary.

For example:

```text
Requirement
    │
    ▼
Identify missing Go knowledge
    │
    ▼
Study the concept
    │
    ▼
Implement the feature
    │
    ▼
Test the implementation
    │
    ▼
Review and refactor
```

This is expected to result in deeper understanding than studying language features without an applied system.

---

## Future Review

This decision should be revisited if:

- Go significantly restricts an important product requirement.
- Development productivity becomes unsustainably low.
- A specific service would materially benefit from another language or ecosystem.
- Operational evidence demonstrates that another technology is better suited to a particular workload.

A future decision to use another language for an individual component does not necessarily invalidate Go as the primary backend language for OshePayment.

Such a decision should be documented in a separate ADR.

## Decision Outcome

Go will be used as the primary backend programming language for OshePayment.

The project accepts slower initial development in exchange for deeper backend learning, explicit system design, and a technology stack that aligns well with the platform's expected long-term architecture.