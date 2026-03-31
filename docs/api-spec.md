# API Spec

All protected endpoints require a valid session token (`Authorization: Bearer ...` or `trainingops_session` cookie).

## Health (Public)

- `GET /healthz`
- `GET /readyz`

## Authentication

- `GET /v1/setup/status` (Public)
- `POST /v1/setup/tenant` (Public, only when tenant count is zero)
Request:
```json
{"tenant_name":"Default Tenant","tenant_slug":"default-tenant","admin_username":"Admin User","admin_email":"admin@example.com","admin_password":"Password123!A"}
```

- `POST /v1/auth/register` (Public)
Request:
```json
{"tenant_id":"tenant-1","tenant_name":"Optional Tenant Name","tenant_slug":"optional-tenant-slug","email":"user@example.com","display_name":"User","password":"Password123!A","role":"learner"}
```
Response:
```json
{"user_id":"...","tenant_id":"tenant-1","email":"u***@example.com","status":"registered"}
```

- `POST /v1/auth/login` (Public)
Request:
```json
{"tenant_id":"tenant-1","email":"user@example.com","password":"Password123!A"}
```
Response:
```json
{"user_id":"...","tenant_id":"tenant-1","email":"u***@example.com","display_name":"User","role":"learner","status":"authenticated","access_token":"...","token_type":"Bearer","expires_at":"...","session_id":"..."}
```

- `POST /v1/auth/logout` (Authenticated)
Response:
```json
{"status":"revoked"}
```

## Workspaces

- `GET /v1/workspaces/:role/dashboard`
Roles:
- `admin` -> `/v1/workspaces/admin/dashboard`
- `coordinator` -> `/v1/workspaces/coordinator/dashboard`
- `instructor` -> `/v1/workspaces/instructor/dashboard`
- `learner` -> `/v1/workspaces/learner/dashboard`

## Bookings

- `POST /v1/bookings/hold` (Coordinator)
Request:
```json
{"room_id":"room-a","instructor_id":"inst-1","title":"Session","start_at":"2026-03-29T10:00:00Z","end_at":"2026-03-29T11:00:00Z","capacity":20,"attendees":18}
```
Response:
```json
{"booking":{...}}
```

- `POST /v1/bookings/confirm` (Coordinator)
Request:
```json
{"booking_id":"..."}
```

- `POST /v1/bookings/cancel` (Coordinator)
Request:
```json
{"booking_id":"..."}
```

- `POST /v1/bookings/reschedule` (Coordinator)
- `PUT /v1/bookings/reschedule` (Coordinator)
Request:
```json
{"booking_id":"...","new_start":"2026-03-30T10:00:00Z","new_end":"2026-03-30T11:00:00Z"}
```

- `POST /v1/bookings/:id/check-in` (Instructor, Coordinator)
Response:
```json
{"status":"checked_in"}
```

## Content

- `POST /v1/uploads/start` (Coordinator, Instructor)
Request:
```json
{"document_id":"uuid","file_name":"manual.pdf","expected_chunks":4,"expected_checksum":"..."}
```

- `POST /v1/uploads/chunk` (Coordinator, Instructor)
Request:
```json
{"session_id":"uuid","index":0,"chunk_b64":"...","checksum":"sha256hex"}
```

- `POST /v1/uploads/finalize` (Coordinator, Instructor)
Request:
```json
{"session_id":"uuid"}
```

- `GET /v1/content/search?q=:query` (Coordinator, Instructor, Learner)
Response:
```json
{"query":"go concurrency","items":[...]}
```

- `GET /v1/content/:id/versions` (Coordinator, Instructor, Learner)
- `POST /v1/content/:id/share` (Coordinator, Instructor)
Request:
```json
{"expiry_hours":72}
```
- `GET /v1/content/share/:token` (Public, expiring share download)

## Learner

- `GET /v1/learner/catalog` (Learner)
- `POST /v1/learner/reserve` (Learner)
Request:
```json
{"booking_id":"...","why":"self-enrollment"}
```
- `GET /v1/learner/my-reservations` (Learner)
- `GET /v1/learner/download/:file_id` (Learner)

## Admin

- `GET /v1/admin/tenants` (Admin)
- `PUT /v1/admin/tenants/:id/policies` (Admin)
Request:
```json
{"policies":{"max_reschedules":2,"cancellation_cutoff_hours":24}}
```
- `GET /v1/admin/users` (Admin)
- `PUT /v1/admin/users/:id/role` (Admin)
Request:
```json
{"role":"coordinator"}
```
- `GET /v1/admin/permissions` (Admin)

## Scheduling

- `POST /v1/schedule/periods` (Admin, Coordinator)
- `GET /v1/schedule/periods` (Admin, Coordinator)
- `POST /v1/schedule/blackout-dates` (Admin, Coordinator)
- `DELETE /v1/schedule/blackout-dates/:id` (Admin, Coordinator)

## Tasks

- `POST /v1/milestones/:id/tasks` (Coordinator)
- `GET /v1/milestones/:id/tasks` (Coordinator, Instructor)
- `PUT /v1/tasks/:id` (Coordinator)
- `DELETE /v1/tasks/:id` (Coordinator)
- `POST /v1/tasks/:id/dependencies` (Coordinator)

## Ingestion

- `POST /v1/ingestion/run` (Admin, Coordinator)

## Analytics

- `GET /v1/analytics/cohorts` (Admin, Coordinator)
- `GET /v1/analytics/anomalies` (Admin, Coordinator)

## Reports

- `GET /v1/reports/bookings.csv` (Admin)
- `GET /v1/reports/compliance.pdf` (Admin)
- `GET /v1/reports/download/:filename` (Admin)
