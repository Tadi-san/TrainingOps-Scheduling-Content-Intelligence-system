# Design

## Concurrency Model

- Booking state changes run through a lock-aware booking repository and service boundary.
- The production path is intended to use database transactions with row-level locking for room, instructor, and overlapping booking rows.
- Temporary booking holds expire after 5 minutes and are auto-released by the expiration worker.
- Task updates use optimistic version checks to prevent lost updates.

## Tenant Isolation

- Every request receives tenant context from the API middleware.
- Repository methods are tenant-scoped by design and filter every query or collection lookup by `tenant_id`.
- Audit records, content, bookings, analytics, and reports are all keyed by tenant.
- PII values are masked or encrypted before they reach logs or persistent state.

## Offline First

- The system avoids external APIs and uses local proxy, user-agent, and ingestion simulation components.
- PDF and CSV reporting are generated locally.
- Dashboard and analytics payloads are served from local repositories or seeded in-memory data.

