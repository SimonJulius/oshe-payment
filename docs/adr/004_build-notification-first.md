# ADR-004: Build the Notification Module First

## Status

Accepted

## Context

OshePayment's business modules need a shared notification capability. All other
business modules depend on Notification, while Notification does not depend on
any of them. This dependency direction determines where module implementation
should begin.

## Decision

Implement the Notification module before all other business modules.

Other modules will use Notification through its public contract. Notification
must remain independent of those business modules and their internal
implementations. It may use shared platform infrastructure.

## Rationale

Building Notification first establishes the capability that all remaining
modules need before those modules are implemented. Because Notification has no
dependencies on other business modules, it can be developed without waiting for
them and provides a foundation for their notification workflows.

## Consequences

- Notification is the first business module in the implementation sequence.
- Subsequent modules can integrate with an established notification contract.
- Dependencies remain one-way, avoiding circular dependencies between
  Notification and the modules that use it.
- This decision establishes the first module to build; it does not prescribe
  the implementation order of the remaining modules.
