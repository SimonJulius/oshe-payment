# OshePayment

OshePayment is a multi-tenant merchant acquiring and payment gateway platform that enables businesses to accept digital payments through hosted checkout, payment links, QR codes, card-not-present transactions, and APIs.

The platform is designed to handle the broader payment lifecycle, including payment processing, platform fees and commissions, merchant balances, settlements, and other acquiring-related capabilities.

## Architecture

OshePayment is initially implemented as a modular monolith with clearly defined domain boundaries.

The architecture is designed so that selected modules can later be extracted into independently deployable services when justified by scaling, reliability, operational, or deployment requirements.

The project intentionally begins as a modular monolith to keep development and operational complexity manageable while the payment domain and service boundaries continue to evolve.#

## Documentation

- [Architecture Overview](./docs/architectures/overview.md)
- [Architecture Decision Records](./docs/adr/)