# TrainingOps Scheduling & Content Intelligence

TrainingOps is a multi-tenant training management platform with scheduling, content workflows, reporting, and role-based access.

## Layout

- `backend/`: Go API service built with Echo
- `frontend/`: React + Vite UI
- `docs/`: Architecture and implementation notes
- `scripts/`: Utility scripts, including trajectory conversion helpers

## Getting started

### Docker

```bash
docker-compose up
```

Services:
- Frontend: `http://localhost:3000`
- Backend: `http://localhost:8080`
- Health: `curl http://localhost:8080/healthz`

### Manual backend

```bash
cd backend
go run ./cmd/api
```

First-run tenant setup (only works when no tenant exists):

```bash
curl -X POST http://localhost:8080/v1/setup/tenant \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_name":"Default Tenant",
    "tenant_slug":"default-tenant",
    "admin_username":"Admin User",
    "admin_email":"admin@example.com",
    "admin_password":"Admin12345678!"
  }'
```

### Manual frontend

```bash
cd frontend
npm ci
npm run dev
```

## Environment variables

- `DATABASE_URL`: Postgres connection string
- `JWT_SECRET`: session signing key (required, min 32 chars)
- `VAULT_MASTER_KEY`: 64-char hex key for PII vault
- `STORAGE_PATH`: local upload storage path (default `./uploads`)
- `REPORTS_PATH`: local reports storage path (default `./reports`)
- `CORS_ALLOWED_ORIGINS`: comma-separated allowed origins
- `ADMIN_SEED_EMAIL`: optional admin email for `seed_db.*`
- `ADMIN_SEED_PASSWORD`: optional admin password for `seed_db.*`

## Tests

```bash
cd backend
go test ./... -v
go test ./internal/api/... -v
make test-integration
```

Integration tests use a dedicated Postgres container:

```bash
cd backend
docker-compose -f docker-compose.test.yml up -d
DATABASE_URL=postgres://postgres:postgres@localhost:5433/trainingops_test?sslmode=disable go test -tags=integration ./... -v
```

```bash
cd frontend
npm run build
npm run test
npm run test:e2e
```

```bash
cd ..
bash run_tests.sh
```

## Verification

```bash
curl http://localhost:8080/healthz
```
