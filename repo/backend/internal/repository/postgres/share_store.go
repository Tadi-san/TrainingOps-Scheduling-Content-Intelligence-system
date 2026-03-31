package postgres

import (
	"context"
	"time"
)

type ShareLinkRecord struct {
	ID          string
	TenantID    string
	DocumentID  string
	Token       string
	ExpiresAt   time.Time
	DownloadCnt int64
}

func (s *Store) SaveShareLink(ctx context.Context, tenantID, documentID, token string, expiresAt, now time.Time) error {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `
		INSERT INTO document_share_links (tenant_id, document_id, token, expires_at, created_at)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5)
		ON CONFLICT (token) DO UPDATE
		SET expires_at = EXCLUDED.expires_at`,
		tenantUUID, documentID, token, expiresAt, now.UTC(),
	)
	return err
}

func (s *Store) GetShareLinkByToken(ctx context.Context, token string) (*ShareLinkRecord, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT id::text, tenant_id::text, document_id::text, token, expires_at, download_count
		FROM document_share_links
		WHERE token = $1`,
		token,
	)
	var record ShareLinkRecord
	if err := row.Scan(&record.ID, &record.TenantID, &record.DocumentID, &record.Token, &record.ExpiresAt, &record.DownloadCnt); err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *Store) IncrementShareDownload(ctx context.Context, token string, now time.Time) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE document_share_links
		SET download_count = download_count + 1,
		    last_download_at = $2
		WHERE token = $1`,
		token, now.UTC(),
	)
	return err
}

func (s *Store) GetLatestUploadedFileForDocument(ctx context.Context, tenantID, documentID string) (*ApprovedFile, error) {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	row := s.Pool.QueryRow(ctx, `
		SELECT id::text, tenant_id::text, file_name, file_path, mime_type
		FROM uploaded_files
		WHERE tenant_id = $1::uuid AND document_id = $2::uuid
		ORDER BY created_at DESC
		LIMIT 1`,
		tenantUUID, documentID,
	)
	var file ApprovedFile
	if err := row.Scan(&file.ID, &file.TenantID, &file.FileName, &file.FilePath, &file.MimeType); err != nil {
		return nil, err
	}
	return &file, nil
}
