# Architecture

This scaffold uses:

- Go + Echo for the API layer
- PostgreSQL for multi-tenant persistent data
- React + Vite for the frontend
- Docker Compose for local orchestration

## Core design notes

- Tenant boundaries are enforced through a tenant-aware request context and repository helpers.
- Authentication includes password complexity checks and lockout controls.
- Sensitive values should be encrypted through the Vault service before persistence.

