package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"trainingops/internal/model"
)

type AdminTenantView struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Slug      string         `json:"slug"`
	Policies  map[string]any `json:"policies"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type AdminUserView struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
}

type PermissionView struct {
	ID  string `json:"id"`
	Key string `json:"key"`
}

type LearnerCatalogFilter struct {
	RoomID       string
	InstructorID string
	From         *time.Time
	To           *time.Time
}

type ApprovedFile struct {
	ID       string
	TenantID string
	FileName string
	FilePath string
	MimeType string
}

func (s *Store) ListTenantsAdmin(ctx context.Context) ([]AdminTenantView, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id::text, name, slug, policies::text, created_at, updated_at
		FROM tenants
		ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []AdminTenantView{}
	for rows.Next() {
		var (
			item        AdminTenantView
			policiesRaw string
		)
		if err := rows.Scan(&item.ID, &item.Name, &item.Slug, &policiesRaw, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Policies = map[string]any{}
		_ = json.Unmarshal([]byte(policiesRaw), &item.Policies)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) UpdateTenantPolicies(ctx context.Context, tenantID string, policies map[string]any) error {
	payload, err := json.Marshal(policies)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `
		UPDATE tenants
		SET policies = $2::jsonb, updated_at = now()
		WHERE id::text = $1 OR slug = $1`,
		tenantID, string(payload),
	)
	return err
}

func (s *Store) ListUsersAdmin(ctx context.Context) ([]AdminUserView, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT u.id::text, u.tenant_id::text, u.email, u.display_name, COALESCE(r.name, 'learner') AS role, u.created_at
		FROM users u
		LEFT JOIN user_roles ur ON ur.tenant_id = u.tenant_id AND ur.user_id = u.id
		LEFT JOIN roles r ON r.id = ur.role_id
		ORDER BY u.created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []AdminUserView{}
	for rows.Next() {
		var item AdminUserView
		if err := rows.Scan(&item.ID, &item.TenantID, &item.Email, &item.DisplayName, &item.Role, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) UpdateUserRole(ctx context.Context, userID, roleName string) error {
	roleName = normalizeRoleName(roleName)
	if roleName == "" {
		return errors.New("invalid role")
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var tenantID string
	if err := tx.QueryRow(ctx, `SELECT tenant_id::text FROM users WHERE id::text = $1`, userID).Scan(&tenantID); err != nil {
		return err
	}
	var roleID string
	if err := tx.QueryRow(ctx, `
		SELECT id::text FROM roles WHERE tenant_id::text = $1 AND name = $2`,
		tenantID, roleName,
	).Scan(&roleID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM user_roles WHERE tenant_id::text = $1 AND user_id::text = $2`, tenantID, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO user_roles (tenant_id, user_id, role_id, created_at)
		VALUES ($1::uuid, $2::uuid, $3::uuid, now())`,
		tenantID, userID, roleID,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) ListPermissions(ctx context.Context) ([]PermissionView, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id::text, permission_key FROM permissions ORDER BY permission_key ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []PermissionView{}
	for rows.Next() {
		var item PermissionView
		if err := rows.Scan(&item.ID, &item.Key); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListRoomsByTenant(ctx context.Context, tenantID string) ([]model.Room, error) {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id::text, tenant_id::text, name, capacity, created_at, updated_at
		FROM rooms
		WHERE tenant_id = $1::uuid
		ORDER BY name ASC`,
		tenantUUID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []model.Room{}
	for rows.Next() {
		var room model.Room
		if err := rows.Scan(&room.ID, &room.TenantID, &room.Name, &room.Capacity, &room.CreatedAt, &room.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, room)
	}
	return items, rows.Err()
}

func (s *Store) CreateRoom(ctx context.Context, tenantID, name string, capacity int) (*model.Room, error) {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	row := s.Pool.QueryRow(ctx, `
		INSERT INTO rooms (tenant_id, name, capacity, created_at, updated_at)
		VALUES ($1::uuid, $2, $3, now(), now())
		RETURNING id::text, tenant_id::text, name, capacity, created_at, updated_at`,
		tenantUUID, name, capacity,
	)
	var room model.Room
	if err := row.Scan(&room.ID, &room.TenantID, &room.Name, &room.Capacity, &room.CreatedAt, &room.UpdatedAt); err != nil {
		return nil, err
	}
	return &room, nil
}

func (s *Store) UpdateRoom(ctx context.Context, tenantID, roomID, name string, capacity int) (*model.Room, error) {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	row := s.Pool.QueryRow(ctx, `
		UPDATE rooms
		SET name = $3, capacity = $4, updated_at = now()
		WHERE tenant_id = $1::uuid AND id = $2::uuid
		RETURNING id::text, tenant_id::text, name, capacity, created_at, updated_at`,
		tenantUUID, roomID, name, capacity,
	)
	var room model.Room
	if err := row.Scan(&room.ID, &room.TenantID, &room.Name, &room.Capacity, &room.CreatedAt, &room.UpdatedAt); err != nil {
		return nil, err
	}
	return &room, nil
}

func (s *Store) ListLearnerCatalog(ctx context.Context, tenantID string, filter LearnerCatalogFilter) ([]model.Booking, error) {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	args := []any{tenantUUID}
	conditions := []string{"tenant_id = $1::uuid", "status = 'confirmed'"}
	if strings.TrimSpace(filter.RoomID) != "" {
		args = append(args, filter.RoomID)
		conditions = append(conditions, fmt.Sprintf("room_id = $%d::uuid", len(args)))
	}
	if strings.TrimSpace(filter.InstructorID) != "" {
		args = append(args, filter.InstructorID)
		conditions = append(conditions, fmt.Sprintf("instructor_id = $%d::uuid", len(args)))
	}
	if filter.From != nil {
		args = append(args, filter.From.UTC())
		conditions = append(conditions, fmt.Sprintf("start_at >= $%d", len(args)))
	}
	if filter.To != nil {
		args = append(args, filter.To.UTC())
		conditions = append(conditions, fmt.Sprintf("end_at <= $%d", len(args)))
	}
	query := `
		SELECT id::text, tenant_id::text, COALESCE(user_id::text, ''), room_id::text, instructor_id::text, title,
		       start_at, end_at, capacity, attendees, status, hold_expires_at, reschedule_count, version, created_at, updated_at,
		       cancelled_at, checked_in_at
		FROM bookings
		WHERE ` + strings.Join(conditions, " AND ") + `
		ORDER BY start_at ASC`
	rows, err := s.Pool.Query(ctx, query, args...)
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

func (s *Store) ReserveLearnerSeat(ctx context.Context, tenantID, bookingID, learnerUserID string, now time.Time) (*model.LearnerReservation, error) {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var capacity, attendees int
	var status string
	if err := tx.QueryRow(ctx, `
		SELECT capacity, attendees, status
		FROM bookings
		WHERE tenant_id = $1::uuid AND id = $2::uuid
		FOR UPDATE`,
		tenantUUID, bookingID,
	).Scan(&capacity, &attendees, &status); err != nil {
		return nil, err
	}
	if status != "confirmed" {
		return nil, errors.New("session is not available for reservations")
	}
	if attendees >= capacity {
		return nil, errors.New("capacity reached")
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO learner_reservations (tenant_id, booking_id, learner_user_id, status, created_at, updated_at)
		VALUES ($1::uuid, $2::uuid, $3::uuid, 'reserved', $4, $4)
		ON CONFLICT (tenant_id, booking_id, learner_user_id) DO NOTHING`,
		tenantUUID, bookingID, learnerUserID, now.UTC(),
	); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE bookings
		SET attendees = attendees + 1, updated_at = $3, version = version + 1
		WHERE tenant_id = $1::uuid AND id = $2::uuid`,
		tenantUUID, bookingID, now.UTC(),
	); err != nil {
		return nil, err
	}

	var reservation model.LearnerReservation
	if err := tx.QueryRow(ctx, `
		SELECT id::text, tenant_id::text, booking_id::text, learner_user_id::text, status, created_at, updated_at
		FROM learner_reservations
		WHERE tenant_id = $1::uuid AND booking_id = $2::uuid AND learner_user_id = $3::uuid`,
		tenantUUID, bookingID, learnerUserID,
	).Scan(&reservation.ID, &reservation.TenantID, &reservation.BookingID, &reservation.LearnerUserID, &reservation.Status, &reservation.CreatedAt, &reservation.UpdatedAt); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &reservation, nil
}

func (s *Store) ListLearnerReservations(ctx context.Context, tenantID, learnerUserID string) ([]model.LearnerReservation, error) {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id::text, tenant_id::text, booking_id::text, learner_user_id::text, status, created_at, updated_at
		FROM learner_reservations
		WHERE tenant_id = $1::uuid AND learner_user_id = $2::uuid
		ORDER BY created_at DESC`,
		tenantUUID, learnerUserID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []model.LearnerReservation{}
	for rows.Next() {
		var item model.LearnerReservation
		if err := rows.Scan(&item.ID, &item.TenantID, &item.BookingID, &item.LearnerUserID, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetApprovedFile(ctx context.Context, tenantID, fileID string) (*ApprovedFile, error) {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	row := s.Pool.QueryRow(ctx, `
		SELECT id::text, tenant_id::text, file_name, file_path, mime_type
		FROM uploaded_files
		WHERE tenant_id = $1::uuid AND id = $2::uuid AND approved = true`,
		tenantUUID, fileID,
	)
	var file ApprovedFile
	if err := row.Scan(&file.ID, &file.TenantID, &file.FileName, &file.FilePath, &file.MimeType); err != nil {
		return nil, err
	}
	return &file, nil
}

func normalizeRoleName(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "administrator", "admin":
		return "admin"
	case "coordinator":
		return "coordinator"
	case "instructor":
		return "instructor"
	case "learner":
		return "learner"
	default:
		return ""
	}
}
