package postgres

import (
	"context"
	"time"

	"trainingops/internal/model"
)

func (s *Store) CreateClassPeriod(ctx context.Context, period model.ClassPeriod) (*model.ClassPeriod, error) {
	tenantUUID, err := s.resolveTenantUUID(ctx, period.TenantID)
	if err != nil {
		return nil, err
	}
	row := s.Pool.QueryRow(ctx, `
		INSERT INTO class_periods (tenant_id, title, start_time, end_time, weekday, created_at, updated_at)
		VALUES ($1::uuid, $2, $3::time, $4::time, $5, now(), now())
		RETURNING id::text, tenant_id::text, title, to_char(start_time, 'HH24:MI:SS'), to_char(end_time, 'HH24:MI:SS'), weekday, created_at, updated_at`,
		tenantUUID, period.Title, period.StartTime, period.EndTime, period.Weekday,
	)
	var created model.ClassPeriod
	if err := row.Scan(&created.ID, &created.TenantID, &created.Title, &created.StartTime, &created.EndTime, &created.Weekday, &created.CreatedAt, &created.UpdatedAt); err != nil {
		return nil, err
	}
	return &created, nil
}

func (s *Store) ListClassPeriods(ctx context.Context, tenantID string) ([]model.ClassPeriod, error) {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id::text, tenant_id::text, title, to_char(start_time, 'HH24:MI:SS'), to_char(end_time, 'HH24:MI:SS'), weekday, created_at, updated_at
		FROM class_periods
		WHERE tenant_id = $1::uuid
		ORDER BY weekday ASC, start_time ASC`, tenantUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var periods []model.ClassPeriod
	for rows.Next() {
		var period model.ClassPeriod
		if err := rows.Scan(&period.ID, &period.TenantID, &period.Title, &period.StartTime, &period.EndTime, &period.Weekday, &period.CreatedAt, &period.UpdatedAt); err != nil {
			return nil, err
		}
		periods = append(periods, period)
	}
	return periods, rows.Err()
}

func (s *Store) UpdateClassPeriod(ctx context.Context, tenantID string, period model.ClassPeriod) (*model.ClassPeriod, error) {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	row := s.Pool.QueryRow(ctx, `
		UPDATE class_periods
		SET title = $3, start_time = $4::time, end_time = $5::time, weekday = $6, updated_at = now()
		WHERE tenant_id = $1::uuid AND id = $2::uuid
		RETURNING id::text, tenant_id::text, title, to_char(start_time, 'HH24:MI:SS'), to_char(end_time, 'HH24:MI:SS'), weekday, created_at, updated_at`,
		tenantUUID, period.ID, period.Title, period.StartTime, period.EndTime, period.Weekday,
	)
	var updated model.ClassPeriod
	if err := row.Scan(&updated.ID, &updated.TenantID, &updated.Title, &updated.StartTime, &updated.EndTime, &updated.Weekday, &updated.CreatedAt, &updated.UpdatedAt); err != nil {
		return nil, err
	}
	return &updated, nil
}

func (s *Store) DeleteClassPeriod(ctx context.Context, tenantID, periodID string) error {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `DELETE FROM class_periods WHERE tenant_id = $1::uuid AND id = $2::uuid`, tenantUUID, periodID)
	return err
}

func (s *Store) CreateBlackoutDate(ctx context.Context, item model.BlackoutDate) (*model.BlackoutDate, error) {
	tenantUUID, err := s.resolveTenantUUID(ctx, item.TenantID)
	if err != nil {
		return nil, err
	}
	row := s.Pool.QueryRow(ctx, `
		INSERT INTO blackout_dates (tenant_id, blackout_date, reason, created_at)
		VALUES ($1::uuid, $2::date, $3, now())
		RETURNING id::text, tenant_id::text, blackout_date, COALESCE(reason, ''), created_at`,
		tenantUUID, item.BlackoutDate, item.Reason,
	)
	var created model.BlackoutDate
	if err := row.Scan(&created.ID, &created.TenantID, &created.BlackoutDate, &created.Reason, &created.CreatedAt); err != nil {
		return nil, err
	}
	return &created, nil
}

func (s *Store) ListBlackoutDates(ctx context.Context, tenantID string) ([]model.BlackoutDate, error) {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id::text, tenant_id::text, blackout_date, COALESCE(reason, ''), created_at
		FROM blackout_dates
		WHERE tenant_id = $1::uuid
		ORDER BY blackout_date ASC`, tenantUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []model.BlackoutDate
	for rows.Next() {
		var item model.BlackoutDate
		if err := rows.Scan(&item.ID, &item.TenantID, &item.BlackoutDate, &item.Reason, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) DeleteBlackoutDate(ctx context.Context, tenantID, blackoutID string) error {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `DELETE FROM blackout_dates WHERE tenant_id = $1::uuid AND id = $2::uuid`, tenantUUID, blackoutID)
	return err
}

func (s *Store) CheckScheduleConflicts(ctx context.Context, tenantID string, startAt, endAt time.Time) (model.ScheduleConflict, error) {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return model.ScheduleConflict{}, err
	}
	reasons := []string{}

	var blackoutCount int
	if err := s.Pool.QueryRow(ctx, `
		SELECT COUNT(1)
		FROM blackout_dates
		WHERE tenant_id = $1::uuid AND blackout_date = $2::date`, tenantUUID, startAt).Scan(&blackoutCount); err != nil {
		return model.ScheduleConflict{}, err
	}
	if blackoutCount > 0 {
		reasons = append(reasons, "date is marked as blackout")
	}

	var periodOverlapCount int
	if err := s.Pool.QueryRow(ctx, `
		SELECT COUNT(1)
		FROM class_periods
		WHERE tenant_id = $1::uuid
		  AND weekday = EXTRACT(DOW FROM $2::timestamptz)::int
		  AND start_time < $4::time
		  AND end_time > $3::time`,
		tenantUUID, startAt, startAt.Format("15:04:05"), endAt.Format("15:04:05"),
	).Scan(&periodOverlapCount); err != nil {
		return model.ScheduleConflict{}, err
	}
	if periodOverlapCount == 0 {
		reasons = append(reasons, "outside configured class periods")
	}

	return model.ScheduleConflict{
		HasConflict: len(reasons) > 0,
		Reasons:     reasons,
	}, nil
}
