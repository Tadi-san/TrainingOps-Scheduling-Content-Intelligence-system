# Delivery Acceptance and Project Architecture Audit

## 1. Verdict
- Partial Pass

## 2. Scope and Verification Boundary
- Reviewed: backend startup path, auth and middleware, booking/content/schedule/task/admin/learner/reporting/ingestion handlers, postgres migrations and repository logic, frontend routes/pages/hooks/styles, and the available backend/frontend tests.
- Executed: `go test ./... -v` in `backend/` succeeded.
- Not executed: Docker Compose startup, DB-backed API/integration tests, frontend build/test/e2e runs, and any container-based verification.
- Docker-based verification was required by the delivery docs but was not executed here because Docker commands are out of scope for this review.
- Remains unconfirmed: live Postgres startup, frontend runtime with installed node modules, and the DB-backed request flows that skip when `DATABASE_URL` is unset.

## 3. Top Findings
- Severity: High
  - Conclusion: Booking concurrency is not strong enough to satisfy the prompt’s “prevent double-booking or oversell” requirement.
  - Brief rationale: `CreateHold` uses a transaction and locks the room row, but the schema has no overlap/exclusion constraint on `bookings`, and the conflict scan only locks existing conflicting rows. That leaves a gap where concurrent holds for the same instructor in different rooms can still race through if no preexisting conflicting row exists.
  - Evidence: [workflow_store.go](C:/Users/Tad/Desktop/EagleAI/EaglePt/fullstack2/backend/internal/repository/postgres/workflow_store.go#L37), [workflow_store.go](C:/Users/Tad/Desktop/EagleAI/EaglePt/fullstack2/backend/internal/repository/postgres/workflow_store.go#L174), [0002_workflows.sql](C:/Users/Tad/Desktop/EagleAI/EaglePt/fullstack2/backend/migrations/0002_workflows.sql#L3)
  - Impact: A core scheduling invariant can fail under contention, which directly undermines the booking/availability promise.
  - Minimum actionable fix: Add a DB-level overlap guard for room/instructor scheduling, or lock the instructor row in the same transaction, and add a regression test for concurrent same-instructor/different-room holds.

- Severity: High
  - Conclusion: Session rotation is effectively disabled, so old session tokens remain valid until expiry.
  - Brief rationale: The rotation path is present but commented out in `AuthGuard`, and the middleware just re-signs the same session claims with the same session ID when it refreshes the cookie. That means the delivery does not actually implement the prompt’s “server-side sessions with rotating tokens” semantics.
  - Evidence: [auth.go](C:/Users/Tad/Desktop/EagleAI/EaglePt/fullstack2/backend/internal/api/middleware/auth.go#L46), [auth.go](C:/Users/Tad/Desktop/EagleAI/EaglePt/fullstack2/backend/internal/api/middleware/auth.go#L86)
  - Impact: A stolen bearer token or cookie remains reusable for the full session lifetime, increasing replay risk and weakening the authentication model described in the prompt.
  - Minimum actionable fix: Restore the rotation branch, generate a fresh session ID on authenticated requests/cookie refresh, revoke the previous session server-side, and add a regression test that proves the old token is rejected.

- Severity: Medium
  - Conclusion: Optimistic concurrency is missing for class-period edits and task dependency updates, contrary to the prompt.
  - Brief rationale: `class_periods` has no version column, `UpdateClassPeriod` writes directly, and `AddDependencies` increments the task version without any expected-version precondition. The prompt explicitly called for optimistic concurrency tokens for calendar rules and task dependencies.
  - Evidence: [0001_init.sql](C:/Users/Tad/Desktop/EagleAI/EaglePt/fullstack2/backend/migrations/0001_init.sql#L68), [schedule.go](C:/Users/Tad/Desktop/EagleAI/EaglePt/fullstack2/backend/internal/api/handlers/schedule.go#L84), [task.go](C:/Users/Tad/Desktop/EagleAI/EaglePt/fullstack2/backend/internal/api/handlers/task.go#L154), [task.go](C:/Users/Tad/Desktop/EagleAI/EaglePt/fullstack2/backend/internal/service/task.go#L70)
  - Impact: Two operators can overwrite each other’s schedule-rule or dependency edits without conflict detection.
  - Minimum actionable fix: Add version fields and conditional updates for class periods, and require `expected_version` for dependency edits the same way task updates already do.

- Severity: Medium
  - Conclusion: The reschedule and cancellation policy is hardcoded instead of being driven by tenant policy, so admin policy changes do not actually affect booking enforcement.
  - Brief rationale: The booking service uses fixed values for hold TTL, 24-hour cancellation cutoff, and a maximum of two reschedules. The admin API exposes tenant policy updates, but the booking rules never read those stored policies.
  - Evidence: [booking.go](C:/Users/Tad/Desktop/EagleAI/EaglePt/fullstack2/backend/internal/service/booking.go#L15), [booking.go](C:/Users/Tad/Desktop/EagleAI/EaglePt/fullstack2/backend/internal/service/booking.go#L24), [booking.go](C:/Users/Tad/Desktop/EagleAI/EaglePt/fullstack2/backend/internal/service/booking.go#L49), [admin.go](C:/Users/Tad/Desktop/EagleAI/EaglePt/fullstack2/backend/internal/api/handlers/admin.go#L27)
  - Impact: Tenant administrators cannot actually tune the booking rules the prompt says are configurable, which weakens prompt fit and operational control.
  - Minimum actionable fix: Read the tenant policy JSON in the booking service and enforce reschedule/cancellation settings from stored policy values instead of hardcoded constants.

## 4. Security Summary
- Authentication: Partial Pass. Local username/password auth, password complexity, lockout, and session cookies exist, but token rotation is disabled in the middleware.
- Route authorization: Pass. Public vs protected routes are separated in the router and `RequireRole` enforces role checks on API endpoints.
- Object-level authorization: Pass. Booking and content handlers perform tenant/ownership checks before object access, with admin overrides where intended.
- Tenant / user isolation: Pass. The request middleware injects tenant context from verified session claims, and repository queries are tenant-scoped.

## 5. Test Sufficiency Summary
- Test Overview
  - Unit tests exist in backend service/security/config packages and frontend hook/component tests.
  - API/integration tests exist under `backend/internal/api` and `backend/internal/repository/postgres`.
  - The documented backend test command ran successfully in this workspace: `go test ./... -v`.
  - DB-backed tests were skipped because `DATABASE_URL` was unset, so the live Postgres path was not exercised here.
- Core Coverage
  - happy path: partial
  - key failure paths: partial
  - security-critical coverage: partial
- Major Gaps
  - DB-backed booking/session/isolation flows were not exercised in this workspace because the tests self-skip without `DATABASE_URL`.
  - Frontend build and e2e execution were not verified because node dependencies were not present locally.
  - No test covers the high-risk concurrency edge where the same instructor is booked in overlapping windows across different rooms.
- Final Test Verdict
  - Partial Pass

## 6. Engineering Quality Summary
- The project is organized like a real product rather than a single-file demo: backend services, handlers, repositories, migrations, and frontend pages/hooks are all separated cleanly.
- Logging, validation, and tenant-aware repository access are present and generally professional.
- The main engineering concern is that a few core business rules are enforced in application code only, with missing DB-level safeguards or hardcoded policy values, which reduces confidence under real concurrency and admin-policy changes.

## 7. Next Actions
- Add a database-level booking overlap guard, plus a regression test for concurrent same-instructor/different-room holds.
- Restore true session/token rotation and revoke old session IDs on refresh.
- Add optimistic concurrency/version checks for class periods and task dependency edits.
- Wire tenant policy values into booking enforcement instead of hardcoded limits.
- Re-run the DB-backed API suite and frontend build/e2e once `DATABASE_URL` and node dependencies are available.
