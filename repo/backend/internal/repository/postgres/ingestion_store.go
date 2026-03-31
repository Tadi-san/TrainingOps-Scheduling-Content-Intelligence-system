package postgres

import (
	"context"
	"time"

	"trainingops/internal/model"
	"trainingops/internal/service"
)

func (s *Store) UpsertIngestionSession(ctx context.Context, session model.IngestionSession) (*model.IngestionSession, error) {
	tenantUUID, err := s.resolveTenantUUID(ctx, session.TenantID)
	if err != nil {
		return nil, err
	}
	row := s.Pool.QueryRow(ctx, `
		INSERT INTO ingestion_sessions (
			id, tenant_id, actor_user_id, proxy, user_agent, request_count, last_seen_at, created_at, updated_at
		) VALUES (
			NULLIF($1, '')::uuid, $2::uuid, NULLIF($3, '')::uuid, $4, $5, $6, $7, now(), now()
		)
		ON CONFLICT (id) DO UPDATE
		SET proxy = EXCLUDED.proxy,
		    user_agent = EXCLUDED.user_agent,
		    request_count = EXCLUDED.request_count,
		    last_seen_at = EXCLUDED.last_seen_at,
		    updated_at = now()
		RETURNING id::text, tenant_id::text, COALESCE(actor_user_id::text, ''), proxy, user_agent, request_count, last_seen_at, created_at, updated_at`,
		session.ID, tenantUUID, session.ActorUserID, session.Proxy, session.UserAgent, session.RequestCount, session.LastSeenAt,
	)
	var out model.IngestionSession
	if err := row.Scan(
		&out.ID, &out.TenantID, &out.ActorUserID, &out.Proxy, &out.UserAgent, &out.RequestCount, &out.LastSeenAt, &out.CreatedAt, &out.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *Store) SaveIngestionJob(ctx context.Context, tenantID, sessionID string, job service.ScraperJob) (*service.ScraperJob, error) {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	row := s.Pool.QueryRow(ctx, `
		INSERT INTO ingestion_jobs (
			id, tenant_id, session_id, url, proxy, user_agent, delay_ms, state, reason, requires_manual_review, created_at, updated_at
		) VALUES (
			NULLIF($1, '')::uuid, $2::uuid, NULLIF($3, '')::uuid, $4, $5, $6, $7, $8, $9, $10, $11, $12
		)
		ON CONFLICT (id) DO UPDATE
		SET state = EXCLUDED.state,
		    reason = EXCLUDED.reason,
		    requires_manual_review = EXCLUDED.requires_manual_review,
		    updated_at = EXCLUDED.updated_at
		RETURNING id::text, url, proxy, user_agent, delay_ms, state, COALESCE(reason, ''), created_at, updated_at`,
		job.ID, tenantUUID, sessionID, job.URL, job.Proxy, job.UserAgent, int(job.Delay.Milliseconds()), string(job.State), job.Reason, job.State == service.ScraperStateManualReview, job.CreatedAt, job.UpdatedAt,
	)
	var out service.ScraperJob
	var delayMS int
	var state string
	if err := row.Scan(&out.ID, &out.URL, &out.Proxy, &out.UserAgent, &delayMS, &state, &out.Reason, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return nil, err
	}
	out.Delay = time.Duration(delayMS) * time.Millisecond
	out.State = service.ScraperState(state)
	return &out, nil
}

func (s *Store) EnqueueManualReview(ctx context.Context, tenantID, jobID, reason string) error {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `
		INSERT INTO manual_review_queue (tenant_id, job_id, reason, status, created_at, updated_at)
		VALUES ($1::uuid, $2::uuid, $3, 'pending', now(), now())
		ON CONFLICT (job_id) DO UPDATE SET reason = EXCLUDED.reason, status = 'pending', updated_at = now()`,
		tenantUUID, jobID, reason,
	)
	return err
}
