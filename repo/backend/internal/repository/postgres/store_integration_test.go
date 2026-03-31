//go:build integration

package postgres

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"trainingops/internal/model"
	"trainingops/internal/security"
	"trainingops/internal/service"
)

func TestCreateHold_ConflictRollsBack(t *testing.T) {
	store, tenantSlug, roomID, instructorID := integrationFixture(t)
	now := time.Now().UTC().Add(3 * time.Hour).Truncate(time.Second)

	_, _, err := store.CreateHold(context.Background(), model.Booking{
		TenantID:     tenantSlug,
		UserID:       "",
		RoomID:       roomID,
		InstructorID: instructorID,
		Title:        "Primary booking",
		StartAt:      now,
		EndAt:        now.Add(time.Hour),
		Capacity:     10,
		Attendees:    8,
	}, 5*time.Minute, time.Now().UTC())
	if err != nil {
		t.Fatalf("expected first hold to succeed: %v", err)
	}

	_, conflicts, err := store.CreateHold(context.Background(), model.Booking{
		TenantID:     tenantSlug,
		UserID:       "",
		RoomID:       roomID,
		InstructorID: instructorID,
		Title:        "Conflicting booking",
		StartAt:      now.Add(10 * time.Minute),
		EndAt:        now.Add(70 * time.Minute),
		Capacity:     10,
		Attendees:    8,
	}, 5*time.Minute, time.Now().UTC())
	if err == nil {
		t.Fatal("expected conflict error")
	}
	if len(conflicts) == 0 {
		t.Fatal("expected conflict reasons")
	}

	bookings, listErr := store.ListBookings(context.Background(), tenantSlug)
	if listErr != nil {
		t.Fatalf("list bookings failed: %v", listErr)
	}
	if len(bookings) != 1 {
		t.Fatalf("expected one booking persisted, got %d", len(bookings))
	}
}

func TestCreateHold_ConcurrentRequestsOnlyOneSucceeds(t *testing.T) {
	store, tenantSlug, roomID, instructorID := integrationFixture(t)
	now := time.Now().UTC().Add(5 * time.Hour).Truncate(time.Second)

	var wg sync.WaitGroup
	wg.Add(2)
	type result struct {
		err error
	}
	results := make(chan result, 2)
	run := func(title string) {
		defer wg.Done()
		_, _, err := store.CreateHold(context.Background(), model.Booking{
			TenantID:     tenantSlug,
			UserID:       "",
			RoomID:       roomID,
			InstructorID: instructorID,
			Title:        title,
			StartAt:      now,
			EndAt:        now.Add(45 * time.Minute),
			Capacity:     10,
			Attendees:    8,
		}, 5*time.Minute, time.Now().UTC())
		results <- result{err: err}
	}
	go run("Concurrent one")
	go run("Concurrent two")
	wg.Wait()
	close(results)

	success := 0
	fail := 0
	for r := range results {
		if r.err == nil {
			success++
		} else {
			fail++
		}
	}
	if success != 1 || fail != 1 {
		t.Fatalf("expected one success and one failure, got success=%d fail=%d", success, fail)
	}
}

func TestCreateUser_EncryptsPIIAtRest(t *testing.T) {
	store, tenantSlug, _, _ := integrationFixture(t)
	vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("vault init failed: %v", err)
	}
	store.Vault = vault

	email := "pii-test@example.com"
	if err := store.CreateUser(context.Background(), model.User{
		TenantID:     tenantSlug,
		Email:        email,
		DisplayName:  "PII Test",
		PasswordHash: "hash",
		Role:         "learner",
	}); err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	tenantUUID, err := store.resolveTenantUUID(context.Background(), tenantSlug)
	if err != nil {
		t.Fatalf("tenant resolve failed: %v", err)
	}
	var encrypted []byte
	if err := store.Pool.QueryRow(context.Background(), `
		SELECT pii_encrypted FROM users
		WHERE tenant_id = $1::uuid AND lower(email) = lower($2)`,
		tenantUUID, email).Scan(&encrypted); err != nil {
		t.Fatalf("query pii_encrypted failed: %v", err)
	}
	if len(encrypted) == 0 {
		t.Fatal("expected encrypted pii bytes")
	}
	if string(encrypted) == email {
		t.Fatal("expected encrypted value to differ from plaintext")
	}
}

func TestSearchContent_ReturnsMatchesByRelevance(t *testing.T) {
	store, tenantSlug, _, _ := integrationFixture(t)
	tenantUUID, err := store.resolveTenantUUID(context.Background(), tenantSlug)
	if err != nil {
		t.Fatalf("tenant resolve failed: %v", err)
	}
	_, err = store.Pool.Exec(context.Background(), `
		INSERT INTO content_items (tenant_id, title, summary)
		VALUES
		  ($1::uuid, 'Go Concurrency Patterns', 'channels and goroutines'),
		  ($1::uuid, 'Python Basics', 'intro course')`, tenantUUID)
	if err != nil {
		t.Fatalf("seed content failed: %v", err)
	}
	items, err := store.SearchContent(context.Background(), tenantSlug, "concurrency")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected at least one search match")
	}
}

func TestWriteAuditTransition_PersistsAuditRow(t *testing.T) {
	store, tenantSlug, _, _ := integrationFixture(t)
	entry := service.AuditEntry{
		ActorUserID: "",
		OldState:    "held",
		NewState:    "confirmed",
		Reason:      "manual verification",
		Who:         "integration test",
		When:        time.Now().UTC().Format(time.RFC3339),
		Action:      "booking_confirmed",
		EntityType:  "booking",
		EntityID:    "booking-123",
		TenantID:    tenantSlug,
		CreatedAt:   time.Now().UTC(),
	}
	if err := store.WriteAuditTransition(context.Background(), entry); err != nil {
		t.Fatalf("write audit failed: %v", err)
	}

	tenantUUID, err := store.resolveTenantUUID(context.Background(), tenantSlug)
	if err != nil {
		t.Fatalf("resolve tenant failed: %v", err)
	}

	var action string
	var metadataRaw string
	if err := store.Pool.QueryRow(context.Background(), `
		SELECT action, metadata::text
		FROM audit_logs
		WHERE tenant_id = $1::uuid AND entity_id = $2
		ORDER BY created_at DESC
		LIMIT 1`, tenantUUID, entry.EntityID).Scan(&action, &metadataRaw); err != nil {
		t.Fatalf("query audit row failed: %v", err)
	}
	if action != "booking_confirmed" {
		t.Fatalf("expected action booking_confirmed, got %s", action)
	}
	metadata := map[string]any{}
	if err := json.Unmarshal([]byte(metadataRaw), &metadata); err != nil {
		t.Fatalf("unmarshal metadata failed: %v", err)
	}
	if metadata["old_state"] != "held" || metadata["new_state"] != "confirmed" {
		t.Fatalf("unexpected state transition metadata: %#v", metadata)
	}
}

func TestHoldExpiryWorker_ReleasesExpiredHoldAndWritesAudit(t *testing.T) {
	store, tenantSlug, roomID, instructorID := integrationFixture(t)
	now := time.Now().UTC()
	created, _, err := store.CreateHold(context.Background(), model.Booking{
		TenantID:     tenantSlug,
		RoomID:       roomID,
		InstructorID: instructorID,
		Title:        "Expiring hold",
		StartAt:      now.Add(2 * time.Hour),
		EndAt:        now.Add(3 * time.Hour),
		Capacity:     20,
		Attendees:    1,
	}, time.Second, now.Add(-2*time.Minute))
	if err != nil {
		t.Fatalf("create hold failed: %v", err)
	}

	recorder := service.NewAuditRecorder(store, slog.Default())
	worker := service.NewHoldExpiryWorker(store, recorder, slog.Default(), 10*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	go worker.Start(ctx)
	time.Sleep(30 * time.Millisecond)
	cancel()
	time.Sleep(10 * time.Millisecond)

	booking, err := store.GetBooking(context.Background(), tenantSlug, created.ID)
	if err != nil {
		t.Fatalf("get booking failed: %v", err)
	}
	if booking.Status != model.BookingStatusExpired {
		t.Fatalf("expected booking status expired, got %s", booking.Status)
	}

	tenantUUID, err := store.resolveTenantUUID(context.Background(), tenantSlug)
	if err != nil {
		t.Fatalf("resolve tenant failed: %v", err)
	}
	var count int
	if err := store.Pool.QueryRow(context.Background(), `
		SELECT COUNT(1) FROM audit_logs
		WHERE tenant_id = $1::uuid AND action = 'booking_expired' AND entity_id = $2`,
		tenantUUID, created.ID).Scan(&count); err != nil {
		t.Fatalf("audit query failed: %v", err)
	}
	if count == 0 {
		t.Fatal("expected booking_expired audit entry")
	}
}

func integrationFixture(t *testing.T) (*Store, string, string, string) {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for integration tests")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("pool create failed: %v", err)
	}
	t.Cleanup(pool.Close)

	store := NewStore(pool, nil)
	suffix := time.Now().UTC().Format("20060102150405")
	tenantSlug := "it-" + suffix
	var tenantID string
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO tenants (name, slug) VALUES ($1, $2) RETURNING id::text`, "IT Tenant "+suffix, tenantSlug).Scan(&tenantID); err != nil {
		t.Fatalf("insert tenant failed: %v", err)
	}

	var roomID string
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO rooms (tenant_id, name, capacity) VALUES ($1::uuid, $2, 20) RETURNING id::text`,
		tenantID, "Room "+suffix).Scan(&roomID); err != nil {
		t.Fatalf("insert room failed: %v", err)
	}

	var instructorID string
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO users (tenant_id, email, display_name, password_hash)
		VALUES ($1::uuid, $2, $3, $4) RETURNING id::text`,
		tenantID, "inst-"+suffix+"@example.com", "Inst "+suffix, "$2a$10$abcdefghijklmnopqrstuv").Scan(&instructorID); err != nil {
		t.Fatalf("insert instructor failed: %v", err)
	}

	return store, tenantSlug, roomID, instructorID
}
