# Questions & Clarifications

This document records questions, ambiguities, and assumptions made during the development of the TrainingOps Scheduling & Content Intelligence system.

---

## 1. User Registration & Tenant Creation

**Question:** The prompt describes "tenant setup" for Administrator, but doesn't specify whether a tenant should exist before user registration, or if the first user automatically creates a tenant.

**Understanding:** For a fully offline-first system, a completely fresh installation needs a way to bootstrap the first admin user and tenant without external intervention.

**Solution:** 
- Added a tenant bootstrap endpoint `POST /v1/setup/tenant` that creates a tenant and admin user in one call.
- This endpoint is only available when no tenants exist in the database (first-run only).
- For subsequent users, registration requires an existing tenant_id (provided by the admin).

**Manual verification:** Run the system fresh → call setup endpoint → login as admin → create additional users/tenants.

---

## 2. Booking Hold Expiry Automation

**Question:** The prompt states a booking is "locked for 5 minutes during checkout and auto-released on timeout." It does not specify whether this should be implemented with a scheduled background job, a database TTL trigger, or client-side polling.

**Understanding:** For reliability in an offline-first environment, the system should not rely solely on client-side timers (which can be tampered with or lost).

**Solution:**
- Implemented a background worker that runs every minute, scanning for expired holds (`status = 'held' AND hold_expires_at < NOW()`).
- Worker releases resources and transitions status to `expired`.
- Frontend displays a countdown timer for user feedback, but backend ensures correctness.

**Manual verification:** Create a hold, wait 5 minutes, refresh page – hold should be released automatically.

---

## 3. Conflict Alternative Suggestions

**Question:** The prompt requires "up to three alternative time slots or rooms" when a booking conflict occurs. Should the alternatives be calculated in real-time, or pre-calculated and cached?

**Understanding:** Real-time calculation is more accurate for availability but could be expensive with many concurrent users. However, given the system is offline-first with limited concurrent load, real-time is acceptable.

**Solution:**
- Implemented a service that, when a conflict is detected, searches for available slots within +/- 3 days of the requested time.
- Alternatives are limited to 3 and include both different times and different rooms (if applicable).
- Results are returned immediately in the conflict response.

**Manual verification:** Try to book an occupied slot → response includes 3 alternative suggestions with timestamps and room names.

---

## 4. Content Duplicate Detection & Merge

**Question:** The prompt mentions "flag duplicates for merge" but doesn't specify whether merging should be automatic or manual, or what fields determine a duplicate.

**Understanding:** Content duplication can be subjective; automatic merging could cause data loss. A guided merge with user approval is safer.

**Solution:**
- Duplicate detection uses file SHA-256 fingerprint and optionally title/author similarity.
- When a duplicate is detected, the UI shows a modal with options: "Keep Both," "Replace Existing," or "Merge Metadata."
- Merge operation combines metadata fields (tags, categories) from both versions.
- Duplicate detection is triggered during upload and in bulk maintenance tools.

**Manual verification:** Upload the same file twice → duplicate modal appears with merge options.

---

## 5. Watermarked Downloads – What Should the Watermark Contain?

**Question:** The prompt requires "watermarked downloads" but doesn't specify the watermark content or placement.

**Understanding:** Watermarks should discourage unauthorized sharing without compromising readability. Standard practice includes downloader identity and timestamp.

**Solution:**
- Watermark includes: downloader's email (masked to first few characters), timestamp of download, and "CONFIDENTIAL" text.
- For PDFs: watermark is a semi-transparent text overlay at a 45° angle across each page.
- For images: watermark is positioned in the bottom-right corner.

**Manual verification:** Download a file after sharing → open file, confirm watermark is visible.

---

## 6. Share Link Expiry – 72 Hours from Generation or First Click?

**Question:** The prompt says share links expire after 72 hours, but doesn't clarify whether expiry counts from link creation time or first access.

**Understanding:** Standard practice is expiry from creation time, preventing links that could be discovered later from lasting indefinitely.

**Solution:**
- Links expire exactly 72 hours after generation, regardless of when they are first accessed.
- Expired links return a 403 error with a message indicating the link has expired.

**Manual verification:** Generate a share link → wait 72 hours (or mock time in test) → try to access → receive 403.

---

## 7. Task Dependencies – How to Handle Circular References?

**Question:** The prompt requires DAG validation for task dependencies but doesn't specify the user experience when a circular dependency is attempted.

**Understanding:** Users should receive immediate feedback when creating a circular dependency, preventing it from being saved.

**Solution:**
- When a user attempts to add a dependency, the backend validates the entire dependency graph for cycles.
- If a cycle would be created, the API returns a 409 Conflict with a message indicating which tasks would form the cycle.
- Frontend displays this error and prevents the dependency from being saved.

**Manual verification:** Create Task A → Task B depends on A → try to make A depend on B → error message appears.

---

## 8. Offline Report Exports – Where Should Files Be Saved?

**Question:** The prompt says "generate local CSV/PDF files to an admin-defined folder" but doesn't specify whether this folder is configurable via environment or UI.

**Understanding:** Admin-defined suggests runtime configuration, but environment variables are simpler for deployment.

**Solution:**
- Folder path is set via `REPORTS_PATH` environment variable (defaults to `./reports`).
- Reports are saved with timestamped filenames: `report_seat_utilization_20260331_143022.csv`.
- An endpoint `GET /v1/reports/download/:filename` allows retrieval of generated files.

**Manual verification:** Generate a report → check `REPORTS_PATH` folder for the file → download via API.

---

## 9. Role-Based UI – Should Missing Features Be Hidden or Disabled?

**Question:** The prompt requires "menu visibility matching permissions" but doesn't specify if unauthorized UI elements should be hidden or just disabled.

**Understanding:** For cleaner UX, elements that a user cannot use should be hidden entirely, not just disabled.

**Solution:**
- Frontend renders menu items and features based on the current user's role.
- Admin sees all tabs, Coordinator sees booking/content/tasks, Learner sees only catalog and reservations.
- Backend still enforces permissions on every API call (defense in depth).

**Manual verification:** Login as Learner → verify admin tabs are not visible in the sidebar.

---

## 10. Anti-Bot Ingestion – What Qualifies as a "Manual Review" Fallback?

**Question:** The prompt mentions CAPTCHA detection triggers a "Manual Review" fallback state, but doesn't specify what manual review entails in an offline system.

**Understanding:** In an offline-first environment, manual review means flagged items appear in a queue for an admin to review and approve/reject.

**Solution:**
- Ingestion service detects CAPTCHA by checking response patterns (too fast, missing tokens, etc.).
- When CAPTCHA is suspected, the ingestion job is paused and moved to a `pending_review` queue.
- Admins can view pending ingestion items via `GET /v1/ingestion/pending` and approve/reject each.
- Approved items continue ingestion; rejected items are discarded with a reason logged.

**Manual verification:** Trigger a high-speed ingestion schedule → check admin panel for pending review items.

---

## Summary of Assumptions

| Area | Assumption Made |
|------|-----------------|
| Tenant creation | First-run setup endpoint, not automatic registration |
| Hold expiry | Background worker, not DB triggers or client-side only |
| Conflict alternatives | Real-time search within +/- 3 days |
| Duplicate merge | Guided merge with user approval, not automatic |
| Watermark | Email (masked) + timestamp + "CONFIDENTIAL" |
| Share link expiry | From creation time, not first access |
| Task dependency cycles | Prevent save with user feedback |
| Report folder | Environment variable, not UI configurable |
| Role-based UI | Hide unauthorized elements, not disable |
| Manual review | Admin approval queue for flagged ingestion jobs |

All assumptions were validated against the prompt's intent and documented for clarity. The implementation prioritizes offline-first operation, security, and user experience.