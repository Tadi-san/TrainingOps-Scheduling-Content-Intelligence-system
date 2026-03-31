package postgres

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"trainingops/internal/model"
)

func (s *Store) GetBooking(ctx context.Context, tenantID, bookingID string) (*model.Booking, error) {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	row := s.Pool.QueryRow(ctx, `
		SELECT id::text, tenant_id::text, COALESCE(user_id::text, ''), room_id::text, instructor_id::text, title,
		       start_at, end_at, capacity, attendees, status, hold_expires_at, reschedule_count, version, created_at, updated_at,
		       cancelled_at, checked_in_at
		FROM bookings
		WHERE tenant_id = $1::uuid AND id = $2::uuid`, tenantUUID, bookingID)
	return scanBooking(row)
}

func (s *Store) CreateHold(ctx context.Context, booking model.Booking, holdTTL time.Duration, now time.Time) (model.Booking, []model.BookingConflict, error) {
	tenantUUID, err := s.resolveTenantUUID(ctx, booking.TenantID)
	if err != nil {
		return model.Booking{}, nil, err
	}

	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return model.Booking{}, nil, err
	}
	defer tx.Rollback(ctx)

	conflicts, err := s.detectBookingConflictsTx(ctx, tx, tenantUUID, booking, now)
	if err != nil {
		return model.Booking{}, nil, err
	}
	if len(conflicts) > 0 {
		return model.Booking{}, conflicts, errors.New("booking conflict")
	}

	var holdExpiresAt *time.Time
	if holdTTL > 0 {
		expires := now.Add(holdTTL)
		holdExpiresAt = &expires
	}

	row := tx.QueryRow(ctx, `
		INSERT INTO bookings (
			tenant_id, user_id, room_id, instructor_id, title,
			start_at, end_at, capacity, attendees, status, hold_expires_at, reschedule_count, version, created_at, updated_at
		) VALUES (
			$1::uuid, NULLIF($2, '')::uuid, $3::uuid, $4::uuid, $5,
			$6, $7, $8, $9, 'held', $10, 0, 1, $11, $11
		)
		RETURNING id::text, tenant_id::text, COALESCE(user_id::text, ''), room_id::text, instructor_id::text, title,
		          start_at, end_at, capacity, attendees, status, hold_expires_at, reschedule_count, version, created_at, updated_at,
		          cancelled_at, checked_in_at`,
		tenantUUID, booking.UserID, booking.RoomID, booking.InstructorID, booking.Title,
		booking.StartAt, booking.EndAt, booking.Capacity, booking.Attendees, holdExpiresAt, now,
	)

	created, err := scanBooking(row)
	if err != nil {
		return model.Booking{}, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return model.Booking{}, nil, err
	}
	return *created, nil, nil
}

func (s *Store) Confirm(ctx context.Context, tenantID, bookingID string, now time.Time) error {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return err
	}
	cmd, err := s.Pool.Exec(ctx, `
		UPDATE bookings
		SET status = 'confirmed', hold_expires_at = NULL, updated_at = $3, version = version + 1
		WHERE tenant_id = $1::uuid
		  AND id = $2::uuid
		  AND status = 'held'
		  AND (hold_expires_at IS NULL OR hold_expires_at > $3)`,
		tenantUUID, bookingID, now,
	)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		_, expireErr := s.Pool.Exec(ctx, `
			UPDATE bookings
			SET status = 'expired', updated_at = $3, version = version + 1
			WHERE tenant_id = $1::uuid
			  AND id = $2::uuid
			  AND status = 'held'
			  AND hold_expires_at <= $3`,
			tenantUUID, bookingID, now,
		)
		if expireErr != nil {
			return expireErr
		}
		return errors.New("booking cannot be confirmed")
	}
	return nil
}

func (s *Store) Cancel(ctx context.Context, tenantID, bookingID string, now time.Time) error {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return err
	}
	cmd, err := s.Pool.Exec(ctx, `
		UPDATE bookings
		SET status = 'cancelled', cancelled_at = $3, updated_at = $3, version = version + 1
		WHERE tenant_id = $1::uuid AND id = $2::uuid`,
		tenantUUID, bookingID, now,
	)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return errors.New("booking not found")
	}
	return nil
}

func (s *Store) CheckIn(ctx context.Context, tenantID, bookingID string, now time.Time) error {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return err
	}
	cmd, err := s.Pool.Exec(ctx, `
		UPDATE bookings
		SET status = 'checked_in', checked_in_at = $3, updated_at = $3, version = version + 1
		WHERE tenant_id = $1::uuid AND id = $2::uuid AND status = 'confirmed'`,
		tenantUUID, bookingID, now,
	)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return errors.New("only confirmed bookings can be checked in")
	}
	return nil
}

func (s *Store) Reschedule(ctx context.Context, tenantID, bookingID string, newStart, newEnd time.Time, now time.Time) (model.Booking, error) {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return model.Booking{}, err
	}

	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return model.Booking{}, err
	}
	defer tx.Rollback(ctx)

	current, err := s.getBookingTx(ctx, tx, tenantUUID, bookingID, true)
	if err != nil {
		return model.Booking{}, err
	}
	current.StartAt = newStart
	current.EndAt = newEnd

	conflicts, err := s.detectBookingConflictsTx(ctx, tx, tenantUUID, *current, now, bookingID)
	if err != nil {
		return model.Booking{}, err
	}
	if len(conflicts) > 0 {
		return model.Booking{}, errors.New("booking conflict")
	}

	row := tx.QueryRow(ctx, `
		UPDATE bookings
		SET start_at = $3, end_at = $4, reschedule_count = reschedule_count + 1, updated_at = $5, version = version + 1
		WHERE tenant_id = $1::uuid AND id = $2::uuid
		RETURNING id::text, tenant_id::text, COALESCE(user_id::text, ''), room_id::text, instructor_id::text, title,
		          start_at, end_at, capacity, attendees, status, hold_expires_at, reschedule_count, version, created_at, updated_at,
		          cancelled_at, checked_in_at`,
		tenantUUID, bookingID, newStart, newEnd, now,
	)
	updated, err := scanBooking(row)
	if err != nil {
		return model.Booking{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return model.Booking{}, err
	}
	return *updated, nil
}

func (s *Store) ReleaseExpiredHolds(ctx context.Context, now time.Time) (int, error) {
	cmd, err := s.Pool.Exec(ctx, `
		UPDATE bookings
		SET status = 'expired', updated_at = $1, version = version + 1
		WHERE status = 'held' AND hold_expires_at IS NOT NULL AND hold_expires_at <= $1`, now)
	if err != nil {
		return 0, err
	}
	return int(cmd.RowsAffected()), nil
}

func (s *Store) ExpireHeldBookings(ctx context.Context, now time.Time) ([]model.Booking, error) {
	rows, err := s.Pool.Query(ctx, `
		UPDATE bookings
		SET status = 'expired', updated_at = $1, version = version + 1
		WHERE status = 'held' AND hold_expires_at IS NOT NULL AND hold_expires_at <= $1
		RETURNING id::text, tenant_id::text, COALESCE(user_id::text, ''), room_id::text, instructor_id::text, title,
		          start_at, end_at, capacity, attendees, status, hold_expires_at, reschedule_count, version, created_at, updated_at,
		          cancelled_at, checked_in_at`,
		now,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []model.Booking{}
	for rows.Next() {
		item, scanErr := scanBooking(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (s *Store) ListBookings(ctx context.Context, tenantID string) ([]model.Booking, error) {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id::text, tenant_id::text, COALESCE(user_id::text, ''), room_id::text, instructor_id::text, title,
		       start_at, end_at, capacity, attendees, status, hold_expires_at, reschedule_count, version, created_at, updated_at,
		       cancelled_at, checked_in_at
		FROM bookings
		WHERE tenant_id = $1::uuid
		ORDER BY start_at ASC`, tenantUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bookings := make([]model.Booking, 0)
	for rows.Next() {
		booking, scanErr := scanBooking(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		bookings = append(bookings, *booking)
	}
	return bookings, rows.Err()
}

func (s *Store) ListRooms(ctx context.Context, tenantID string) ([]model.Room, error) {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id::text, tenant_id::text, name, capacity, created_at, updated_at
		FROM rooms
		WHERE tenant_id = $1::uuid
		ORDER BY name ASC`, tenantUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rooms := []model.Room{}
	for rows.Next() {
		var room model.Room
		if err := rows.Scan(&room.ID, &room.TenantID, &room.Name, &room.Capacity, &room.CreatedAt, &room.UpdatedAt); err != nil {
			return nil, err
		}
		rooms = append(rooms, room)
	}
	return rooms, rows.Err()
}

func (s *Store) ListInstructors(ctx context.Context, tenantID string) ([]model.Instructor, error) {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT DISTINCT u.id::text, u.tenant_id::text, u.display_name, u.created_at, u.updated_at
		FROM users u
		LEFT JOIN user_roles ur ON ur.tenant_id = u.tenant_id AND ur.user_id = u.id
		LEFT JOIN roles r ON r.id = ur.role_id
		WHERE u.tenant_id = $1::uuid
		  AND (lower(COALESCE(r.name, '')) = 'instructor')
		ORDER BY u.display_name ASC`, tenantUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	instructors := []model.Instructor{}
	for rows.Next() {
		var instructor model.Instructor
		if err := rows.Scan(&instructor.ID, &instructor.TenantID, &instructor.Name, &instructor.CreatedAt, &instructor.UpdatedAt); err != nil {
			return nil, err
		}
		instructors = append(instructors, instructor)
	}
	return instructors, rows.Err()
}

func (s *Store) UpsertContentItem(ctx context.Context, item model.ContentItem) error {
	tenantUUID, err := s.resolveTenantUUID(ctx, item.TenantID)
	if err != nil {
		return err
	}
	var category any
	if strings.TrimSpace(item.CategoryID) != "" {
		category = item.CategoryID
	}
	_, err = s.Pool.Exec(ctx, `
		INSERT INTO content_items (id, tenant_id, category_id, title, summary, checksum, version, created_by_user_id, created_at, updated_at)
		VALUES (NULLIF($1, '')::uuid, $2::uuid, NULLIF($3, '')::uuid, $4, '', $5, $6, NULLIF($7, '')::uuid, now(), now())
		ON CONFLICT (id) DO UPDATE
		SET title = EXCLUDED.title,
		    checksum = EXCLUDED.checksum,
		    version = EXCLUDED.version,
		    category_id = EXCLUDED.category_id,
		    updated_at = now()`,
		item.ID, tenantUUID, category, item.Title, item.Checksum, item.Version, item.CreatedByUserID,
	)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `
		INSERT INTO content_metadata (tenant_id, content_item_id, difficulty, duration_minutes, created_at, updated_at)
		SELECT $1::uuid, ci.id, $2, $3, now(), now()
		FROM content_items ci
		WHERE ci.tenant_id = $1::uuid AND lower(ci.title) = lower($4)
		ORDER BY ci.updated_at DESC
		LIMIT 1
		ON CONFLICT (tenant_id, content_item_id) DO UPDATE
		SET difficulty = EXCLUDED.difficulty,
		    duration_minutes = EXCLUDED.duration_minutes,
		    updated_at = now()`,
		tenantUUID, item.Difficulty, item.DurationMinutes, item.Title,
	)
	return err
}

func (s *Store) DeleteContentItem(ctx context.Context, tenantID, itemID string) error {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `DELETE FROM content_items WHERE tenant_id = $1::uuid AND id = $2::uuid`, tenantUUID, itemID)
	return err
}

func (s *Store) ListContentItems(ctx context.Context, tenantID string) ([]model.ContentItem, error) {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT ci.id::text, ci.tenant_id::text, COALESCE(ci.category_id::text, ''), ci.title, COALESCE(cm.difficulty, 1), COALESCE(cm.duration_minutes, 5),
		       ci.version, COALESCE(ci.checksum, ''), COALESCE(ci.created_by_user_id::text, ''), ci.created_at, ci.updated_at
		FROM content_items ci
		LEFT JOIN content_metadata cm ON cm.tenant_id = ci.tenant_id AND cm.content_item_id = ci.id
		WHERE ci.tenant_id = $1::uuid
		ORDER BY ci.updated_at DESC`, tenantUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []model.ContentItem{}
	for rows.Next() {
		var item model.ContentItem
		if err := rows.Scan(
			&item.ID, &item.TenantID, &item.CategoryID, &item.Title, &item.Difficulty, &item.DurationMinutes,
			&item.Version, &item.Checksum, &item.CreatedByUserID, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) SearchContent(ctx context.Context, tenantID, query string) ([]model.ContentItem, error) {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT ci.id::text, ci.tenant_id::text, COALESCE(ci.category_id::text, ''), ci.title, COALESCE(cm.difficulty, 1), COALESCE(cm.duration_minutes, 5),
		       ci.version, COALESCE(ci.checksum, ''), COALESCE(ci.created_by_user_id::text, ''), ci.created_at, ci.updated_at
		FROM content_items ci
		LEFT JOIN content_metadata cm ON cm.tenant_id = ci.tenant_id AND cm.content_item_id = ci.id
		WHERE ci.tenant_id = $1::uuid
		  AND ci.search_vector @@ plainto_tsquery('english', $2)
		ORDER BY ts_rank(ci.search_vector, plainto_tsquery('english', $2)) DESC, ci.updated_at DESC
		LIMIT 50`, tenantUUID, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []model.ContentItem{}
	for rows.Next() {
		var item model.ContentItem
		if err := rows.Scan(
			&item.ID, &item.TenantID, &item.CategoryID, &item.Title, &item.Difficulty, &item.DurationMinutes,
			&item.Version, &item.Checksum, &item.CreatedByUserID, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) UpsertTag(ctx context.Context, tag model.ContentTag) error {
	tenantUUID, err := s.resolveTenantUUID(ctx, tag.TenantID)
	if err != nil {
		return err
	}
	normalized := strings.ToLower(strings.TrimSpace(tag.Name))
	_, err = s.Pool.Exec(ctx, `
		INSERT INTO content_tags (id, tenant_id, name, normalized_name, created_at, updated_at)
		VALUES (NULLIF($1, '')::uuid, $2::uuid, $3, $4, now(), now())
		ON CONFLICT (tenant_id, normalized_name) DO UPDATE SET name = EXCLUDED.name, updated_at = now()`,
		tag.ID, tenantUUID, tag.Name, normalized,
	)
	return err
}

func (s *Store) DeleteTag(ctx context.Context, tenantID, tagID string) error {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `DELETE FROM content_tags WHERE tenant_id = $1::uuid AND id = $2::uuid`, tenantUUID, tagID)
	return err
}

func (s *Store) ListTags(ctx context.Context, tenantID string) ([]model.ContentTag, error) {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id::text, tenant_id::text, name, created_at, updated_at
		FROM content_tags
		WHERE tenant_id = $1::uuid
		ORDER BY name ASC`, tenantUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tags := []model.ContentTag{}
	for rows.Next() {
		var tag model.ContentTag
		if err := rows.Scan(&tag.ID, &tag.TenantID, &tag.Name, &tag.CreatedAt, &tag.UpdatedAt); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

func (s *Store) SaveDocument(ctx context.Context, document model.Document) error {
	return s.UpsertContentItem(ctx, model.ContentItem{
		ID:              document.ID,
		TenantID:        document.TenantID,
		Title:           document.Title,
		Version:         int64(document.CurrentVersion),
		Checksum:        document.Checksum,
		DurationMinutes: 5,
		Difficulty:      1,
	})
}

func (s *Store) ListDocuments(ctx context.Context, tenantID string) ([]model.Document, error) {
	items, err := s.ListContentItems(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]model.Document, 0, len(items))
	for _, item := range items {
		out = append(out, model.Document{
			ID:             item.ID,
			TenantID:       item.TenantID,
			Title:          item.Title,
			CurrentVersion: int(item.Version),
			Checksum:       item.Checksum,
			CreatedAt:      item.CreatedAt,
			UpdatedAt:      item.UpdatedAt,
		})
	}
	return out, nil
}

func (s *Store) AddDocumentVersion(ctx context.Context, version model.DocumentVersion) (model.DocumentVersion, error) {
	tenantUUID, err := s.resolveTenantUUID(ctx, version.TenantID)
	if err != nil {
		return model.DocumentVersion{}, err
	}
	body := map[string]any{
		"file_name":  version.FileName,
		"checksum":   version.Checksum,
		"size_bytes": version.SizeBytes,
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return model.DocumentVersion{}, err
	}

	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return model.DocumentVersion{}, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `SELECT 1 FROM content_items WHERE tenant_id = $1::uuid AND id = $2::uuid FOR UPDATE`, tenantUUID, version.DocumentID); err != nil {
		return model.DocumentVersion{}, err
	}

	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(version_number), 0) + 1
		FROM document_versions
		WHERE tenant_id = $1::uuid AND content_item_id = $2::uuid`,
		tenantUUID, version.DocumentID,
	).Scan(&version.Version); err != nil {
		return model.DocumentVersion{}, err
	}

	if err := tx.QueryRow(ctx, `
		INSERT INTO document_versions (id, tenant_id, content_item_id, version_number, body, created_at)
		VALUES (NULLIF($1, '')::uuid, $2::uuid, $3::uuid, $4, $5, $6)
		RETURNING id::text, tenant_id::text, content_item_id::text, version_number, created_at`,
		version.ID, tenantUUID, version.DocumentID, version.Version, string(bodyBytes), version.CreatedAt,
	).Scan(&version.ID, &version.TenantID, &version.DocumentID, &version.Version, &version.CreatedAt); err != nil {
		return model.DocumentVersion{}, err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE content_items
		SET version = $3, updated_at = now()
		WHERE tenant_id = $1::uuid AND id = $2::uuid`,
		tenantUUID, version.DocumentID, version.Version,
	); err != nil {
		return model.DocumentVersion{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return model.DocumentVersion{}, err
	}
	return version, nil
}

func (s *Store) ListDocumentVersions(ctx context.Context, tenantID, documentID string) ([]model.DocumentVersion, error) {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id::text, tenant_id::text, content_item_id::text, version_number, body, created_at
		FROM document_versions
		WHERE tenant_id = $1::uuid AND content_item_id = $2::uuid
		ORDER BY version_number ASC`, tenantUUID, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	versions := []model.DocumentVersion{}
	for rows.Next() {
		var (
			version model.DocumentVersion
			bodyRaw string
		)
		if err := rows.Scan(&version.ID, &version.TenantID, &version.DocumentID, &version.Version, &bodyRaw, &version.CreatedAt); err != nil {
			return nil, err
		}
		body := map[string]any{}
		_ = json.Unmarshal([]byte(bodyRaw), &body)
		if fileName, ok := body["file_name"].(string); ok {
			version.FileName = fileName
		}
		if checksum, ok := body["checksum"].(string); ok {
			version.Checksum = checksum
		}
		switch size := body["size_bytes"].(type) {
		case float64:
			version.SizeBytes = int64(size)
		case int64:
			version.SizeBytes = size
		}
		versions = append(versions, version)
	}
	return versions, rows.Err()
}

func (s *Store) SaveUploadSession(ctx context.Context, session model.UploadSession) error {
	tenantUUID, err := s.resolveTenantUUID(ctx, session.TenantID)
	if err != nil {
		return err
	}
	payload, err := marshalJSON(session.ReceivedChunks)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `
		INSERT INTO upload_sessions (
			id, tenant_id, document_id, file_name, expected_chunks, expected_checksum, received_chunks, created_at, expires_at, updated_at
		) VALUES (
			NULLIF($1, '')::uuid, $2::uuid, NULLIF($3, '')::uuid, $4, $5, $6, $7::jsonb, $8, $9, $10
		)
		ON CONFLICT (id) DO UPDATE
		SET received_chunks = EXCLUDED.received_chunks,
		    expected_checksum = EXCLUDED.expected_checksum,
		    expected_chunks = EXCLUDED.expected_chunks,
		    updated_at = EXCLUDED.updated_at`,
		session.ID, tenantUUID, session.DocumentID, session.FileName, session.ExpectedChunks, session.ExpectedChecksum,
		string(payload), session.CreatedAt, session.ExpiresAt, session.UpdatedAt,
	)
	return err
}

func (s *Store) GetUploadSession(ctx context.Context, tenantID, sessionID string) (*model.UploadSession, error) {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	row := s.Pool.QueryRow(ctx, `
		SELECT id::text, tenant_id::text, document_id::text, file_name, expected_chunks, expected_checksum,
		       received_chunks::text, created_at, expires_at, updated_at
		FROM upload_sessions
		WHERE tenant_id = $1::uuid AND id = $2::uuid`,
		tenantUUID, sessionID,
	)
	var session model.UploadSession
	var receivedRaw string
	if err := row.Scan(
		&session.ID, &session.TenantID, &session.DocumentID, &session.FileName, &session.ExpectedChunks, &session.ExpectedChecksum,
		&receivedRaw, &session.CreatedAt, &session.ExpiresAt, &session.UpdatedAt,
	); err != nil {
		return nil, err
	}
	session.ReceivedChunks = map[int][]byte{}
	if strings.TrimSpace(receivedRaw) != "" {
		var raw map[string]string
		if err := json.Unmarshal([]byte(receivedRaw), &raw); err == nil {
			for k, v := range raw {
				var idx int
				if _, err := fmt.Sscanf(k, "%d", &idx); err == nil {
					decoded, decErr := base64.StdEncoding.DecodeString(v)
					if decErr != nil {
						continue
					}
					session.ReceivedChunks[idx] = decoded
				}
			}
		}
	}
	return &session, nil
}

func (s *Store) DeleteUploadSession(ctx context.Context, tenantID, sessionID string) error {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `DELETE FROM upload_sessions WHERE tenant_id = $1::uuid AND id = $2::uuid`, tenantUUID, sessionID)
	return err
}

func (s *Store) UpsertTask(ctx context.Context, task model.Task) error {
	tenantUUID, err := s.resolveTenantUUID(ctx, task.TenantID)
	if err != nil {
		return err
	}
	deps, err := marshalJSON(task.DependencyIDs)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `
		INSERT INTO tasks (
			id, tenant_id, milestone_id, parent_task_id, title, description, due_date, status, dependency_ids,
			estimated_minutes, actual_minutes, version, created_at, updated_at
		) VALUES (
			NULLIF($1, '')::uuid, $2::uuid, NULLIF($3, '')::uuid, NULLIF($4, '')::uuid, $5, $6, $7, 'todo', $8::jsonb,
			$9, $10, $11, $12, $13
		)
		ON CONFLICT (id) DO UPDATE
		SET title = EXCLUDED.title,
		    description = EXCLUDED.description,
		    due_date = EXCLUDED.due_date,
		    dependency_ids = EXCLUDED.dependency_ids,
		    estimated_minutes = EXCLUDED.estimated_minutes,
		    actual_minutes = EXCLUDED.actual_minutes,
		    version = EXCLUDED.version,
		    updated_at = EXCLUDED.updated_at`,
		task.ID, tenantUUID, task.MilestoneID, task.ParentTaskID, task.Title, task.Description, task.DueDate, string(deps),
		task.EstimatedMinutes, task.ActualMinutes, task.Version, task.CreatedAt, task.UpdatedAt,
	)
	return err
}

func (s *Store) GetTask(ctx context.Context, tenantID, taskID string) (*model.Task, error) {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	row := s.Pool.QueryRow(ctx, `
		SELECT id::text, tenant_id::text, COALESCE(milestone_id::text, ''), COALESCE(parent_task_id::text, ''), title, COALESCE(description, ''),
		       due_date, dependency_ids::text, estimated_minutes, actual_minutes, version, created_at, updated_at
		FROM tasks
		WHERE tenant_id = $1::uuid AND id = $2::uuid`,
		tenantUUID, taskID,
	)
	return scanTask(row)
}

func (s *Store) ListTasks(ctx context.Context, tenantID string) ([]model.Task, error) {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id::text, tenant_id::text, COALESCE(milestone_id::text, ''), COALESCE(parent_task_id::text, ''), title, COALESCE(description, ''),
		       due_date, dependency_ids::text, estimated_minutes, actual_minutes, version, created_at, updated_at
		FROM tasks
		WHERE tenant_id = $1::uuid
		ORDER BY updated_at DESC`, tenantUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := []model.Task{}
	for rows.Next() {
		task, scanErr := scanTask(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		tasks = append(tasks, *task)
	}
	return tasks, rows.Err()
}

func (s *Store) DeleteTask(ctx context.Context, tenantID, taskID string) error {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `DELETE FROM tasks WHERE tenant_id = $1::uuid AND id = $2::uuid`, tenantUUID, taskID)
	return err
}

func (s *Store) CreateBooking(ctx context.Context, booking model.Booking, holdTTL time.Duration, now time.Time) (model.Booking, []model.BookingConflict, error) {
	return s.CreateHold(ctx, booking, holdTTL, now)
}

func (s *Store) UpdateBooking(ctx context.Context, tenantID, bookingID string, startAt, endAt time.Time, now time.Time) (model.Booking, error) {
	return s.Reschedule(ctx, tenantID, bookingID, startAt, endAt, now)
}

func (s *Store) DeleteBooking(ctx context.Context, tenantID, bookingID string, now time.Time) error {
	return s.Cancel(ctx, tenantID, bookingID, now)
}

func (s *Store) detectBookingConflictsTx(ctx context.Context, tx pgx.Tx, tenantUUID string, candidate model.Booking, now time.Time, excludeBookingIDs ...string) ([]model.BookingConflict, error) {
	roomRow := tx.QueryRow(ctx, `SELECT capacity FROM rooms WHERE tenant_id = $1::uuid AND id = $2::uuid FOR UPDATE`, tenantUUID, candidate.RoomID)
	roomCapacity := 0
	if err := roomRow.Scan(&roomCapacity); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return []model.BookingConflict{{Reason: model.BookingConflictRoom, Detail: "room does not exist"}}, nil
		}
		return nil, err
	}
	conflicts := []model.BookingConflict{}
	if roomCapacity > 0 && candidate.Attendees > roomCapacity {
		conflicts = append(conflicts, model.BookingConflict{
			Reason: model.BookingConflictCapacity,
			Detail: fmt.Sprintf("room capacity %d is lower than attendees %d", roomCapacity, candidate.Attendees),
		})
	}

	var blackoutCount int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(1)
		FROM blackout_dates
		WHERE tenant_id = $1::uuid AND blackout_date = $2::date`,
		tenantUUID, candidate.StartAt,
	).Scan(&blackoutCount); err != nil {
		return nil, err
	}
	if blackoutCount > 0 {
		conflicts = append(conflicts, model.BookingConflict{
			Reason: model.BookingConflictRoom,
			Detail: "requested date is blocked by blackout schedule",
		})
	}

	var periodOverlapCount int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(1)
		FROM class_periods
		WHERE tenant_id = $1::uuid
		  AND weekday = EXTRACT(DOW FROM $2::timestamptz)::int
		  AND start_time <= $3::time
		  AND end_time >= $4::time`,
		tenantUUID, candidate.StartAt, candidate.StartAt.Format("15:04:05"), candidate.EndAt.Format("15:04:05"),
	).Scan(&periodOverlapCount); err != nil {
		return nil, err
	}
	var configuredPeriodCount int
	if err := tx.QueryRow(ctx, `SELECT COUNT(1) FROM class_periods WHERE tenant_id = $1::uuid`, tenantUUID).Scan(&configuredPeriodCount); err != nil {
		return nil, err
	}
	if configuredPeriodCount > 0 && periodOverlapCount == 0 {
		conflicts = append(conflicts, model.BookingConflict{
			Reason: model.BookingConflictRoom,
			Detail: "requested time is outside configured class periods",
		})
	}

	query := `
		SELECT id::text, room_id::text, instructor_id::text, hold_expires_at
		FROM bookings
		WHERE tenant_id = $1::uuid
		  AND status NOT IN ('cancelled','expired')
		  AND start_at < $3
		  AND end_at > $2
		  AND (room_id = $4::uuid OR instructor_id = $5::uuid)
		FOR UPDATE`
	rows, err := tx.Query(ctx, query, tenantUUID, candidate.StartAt, candidate.EndAt, candidate.RoomID, candidate.InstructorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	exclude := map[string]bool{}
	for _, id := range excludeBookingIDs {
		exclude[id] = true
	}
	for rows.Next() {
		var id, roomID, instructorID string
		var holdExpiresAt *time.Time
		if err := rows.Scan(&id, &roomID, &instructorID, &holdExpiresAt); err != nil {
			return nil, err
		}
		if exclude[id] {
			continue
		}
		if holdExpiresAt != nil && now.After(*holdExpiresAt) {
			continue
		}
		if roomID == candidate.RoomID {
			conflicts = append(conflicts, model.BookingConflict{Reason: model.BookingConflictRoom, Detail: "room is already booked in the requested window"})
		}
		if instructorID == candidate.InstructorID {
			conflicts = append(conflicts, model.BookingConflict{Reason: model.BookingConflictInstructor, Detail: "instructor is already booked in the requested window"})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return dedupeBookingConflicts(conflicts), nil
}

func dedupeBookingConflicts(conflicts []model.BookingConflict) []model.BookingConflict {
	seen := map[model.BookingConflictReason]bool{}
	out := []model.BookingConflict{}
	for _, conflict := range conflicts {
		if seen[conflict.Reason] {
			continue
		}
		seen[conflict.Reason] = true
		out = append(out, conflict)
	}
	return out
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanBooking(row rowScanner) (*model.Booking, error) {
	var booking model.Booking
	var holdExpiresAt *time.Time
	var cancelledAt *time.Time
	var checkedInAt *time.Time
	var status string
	if err := row.Scan(
		&booking.ID, &booking.TenantID, &booking.UserID, &booking.RoomID, &booking.InstructorID, &booking.Title,
		&booking.StartAt, &booking.EndAt, &booking.Capacity, &booking.Attendees, &status, &holdExpiresAt,
		&booking.RescheduleCount, &booking.Version, &booking.CreatedAt, &booking.UpdatedAt,
		&cancelledAt, &checkedInAt,
	); err != nil {
		return nil, err
	}
	booking.Status = model.BookingStatus(status)
	booking.HoldExpiresAt = holdExpiresAt
	booking.CancelledAt = cancelledAt
	booking.CheckedInAt = checkedInAt
	return &booking, nil
}

func (s *Store) getBookingTx(ctx context.Context, tx pgx.Tx, tenantUUID, bookingID string, forUpdate bool) (*model.Booking, error) {
	query := `
		SELECT id::text, tenant_id::text, COALESCE(user_id::text, ''), room_id::text, instructor_id::text, title,
		       start_at, end_at, capacity, attendees, status, hold_expires_at, reschedule_count, version, created_at, updated_at,
		       cancelled_at, checked_in_at
		FROM bookings
		WHERE tenant_id = $1::uuid AND id = $2::uuid`
	if forUpdate {
		query += " FOR UPDATE"
	}
	return scanBooking(tx.QueryRow(ctx, query, tenantUUID, bookingID))
}

func scanTask(row rowScanner) (*model.Task, error) {
	var task model.Task
	var depsRaw string
	var dueDate *time.Time
	if err := row.Scan(
		&task.ID, &task.TenantID, &task.MilestoneID, &task.ParentTaskID, &task.Title, &task.Description,
		&dueDate, &depsRaw, &task.EstimatedMinutes, &task.ActualMinutes, &task.Version, &task.CreatedAt, &task.UpdatedAt,
	); err != nil {
		return nil, err
	}
	task.DueDate = dueDate
	if strings.TrimSpace(depsRaw) != "" {
		_ = json.Unmarshal([]byte(depsRaw), &task.DependencyIDs)
	}
	if task.DependencyIDs == nil {
		task.DependencyIDs = []string{}
	}
	return &task, nil
}

func marshalJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}
