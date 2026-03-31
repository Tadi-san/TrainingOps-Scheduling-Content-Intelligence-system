package handlers

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"trainingops/internal/model"
	"trainingops/internal/repository/postgres"
	"trainingops/internal/service"
	"trainingops/internal/tenant"
)

type ContentHandler struct {
	Content *service.ContentService
}

type contentSearcher interface {
	SearchContent(ctx context.Context, tenantID, query string) ([]model.ContentItem, error)
}

type versionLister interface {
	ListDocumentVersions(ctx context.Context, tenantID, documentID string) ([]model.DocumentVersion, error)
}

type shareStore interface {
	SaveShareLink(ctx context.Context, tenantID, documentID, token string, expiresAt, now time.Time) error
	GetShareLinkByToken(ctx context.Context, token string) (*postgres.ShareLinkRecord, error)
	IncrementShareDownload(ctx context.Context, token string, now time.Time) error
	GetLatestUploadedFileForDocument(ctx context.Context, tenantID, documentID string) (*postgres.ApprovedFile, error)
}

type generateShareRequest struct {
	ExpiryHours int `json:"expiry_hours"`
}

type startUploadRequest struct {
	DocumentID       string `json:"document_id"`
	FileName         string `json:"file_name"`
	ExpectedChunks   int    `json:"expected_chunks"`
	ExpectedChecksum string `json:"expected_checksum"`
}

type appendChunkRequest struct {
	SessionID string `json:"session_id"`
	Index     int    `json:"index"`
	ChunkB64  string `json:"chunk_b64"`
	Checksum  string `json:"checksum"`
}

type finalizeUploadRequest struct {
	SessionID string `json:"session_id"`
}

func (h *ContentHandler) StartUpload(c echo.Context) error {
	var req startUploadRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok || tenantID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "tenant context required"})
	}
	req.DocumentID = strings.TrimSpace(req.DocumentID)
	req.FileName = strings.TrimSpace(req.FileName)
	if req.DocumentID == "" || req.FileName == "" || req.ExpectedChunks <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "document_id, file_name, and expected_chunks are required"})
	}
	if err := h.ensureOrCreateContentOwnership(c, req.DocumentID, req.FileName); err != nil {
		return jsonError(c, http.StatusForbidden, "Access denied")
	}

	session := model.UploadSession{
		ID:               newUUIDString(),
		TenantID:         tenantID,
		DocumentID:       req.DocumentID,
		FileName:         req.FileName,
		ExpectedChunks:   req.ExpectedChunks,
		ExpectedChecksum: req.ExpectedChecksum,
	}
	if err := h.Content.StartUpload(c.Request().Context(), session); err != nil {
		return jsonError(c, http.StatusBadRequest, "Unable to start upload")
	}

	return c.JSON(http.StatusCreated, session)
}

func (h *ContentHandler) AppendChunk(c echo.Context) error {
	var req appendChunkRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok || tenantID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "tenant context required"})
	}
	req.SessionID = strings.TrimSpace(req.SessionID)
	if req.SessionID == "" || req.ChunkB64 == "" || req.Checksum == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "session_id, chunk_b64, and checksum are required"})
	}
	session, err := h.Content.Store.GetUploadSession(c.Request().Context(), tenantID, req.SessionID)
	if err != nil {
		return jsonError(c, http.StatusNotFound, "Upload session not found")
	}
	if err := h.ensureContentOwnership(c, session.DocumentID); err != nil {
		return jsonError(c, http.StatusForbidden, "Access denied")
	}

	chunk, err := base64.StdEncoding.DecodeString(req.ChunkB64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "chunk_b64 must be valid base64"})
	}

	complete, err := h.Content.AppendChunk(c.Request().Context(), tenantID, req.SessionID, req.Index, chunk, req.Checksum)
	if err != nil {
		return jsonError(c, http.StatusBadRequest, "Invalid upload chunk")
	}

	return c.JSON(http.StatusOK, map[string]any{
		"complete": complete,
	})
}

func (h *ContentHandler) FinalizeUpload(c echo.Context) error {
	var req finalizeUploadRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok || tenantID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "tenant context required"})
	}
	req.SessionID = strings.TrimSpace(req.SessionID)
	if req.SessionID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "session_id is required"})
	}
	session, err := h.Content.Store.GetUploadSession(c.Request().Context(), tenantID, req.SessionID)
	if err != nil {
		return jsonError(c, http.StatusNotFound, "Upload session not found")
	}
	if err := h.ensureContentOwnership(c, session.DocumentID); err != nil {
		return jsonError(c, http.StatusForbidden, "Access denied")
	}

	version, err := h.Content.FinalizeUpload(c.Request().Context(), tenantID, req.SessionID, time.Now().UTC())
	if err != nil {
		return jsonError(c, http.StatusConflict, "Upload is incomplete or invalid")
	}

	return c.JSON(http.StatusOK, version)
}

func (h *ContentHandler) Search(c echo.Context) error {
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "tenant context required"})
	}
	query := strings.TrimSpace(c.QueryParam("q"))
	if query == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "query is required"})
	}
	searcher, ok := h.Content.Store.(contentSearcher)
	if !ok {
		return c.JSON(http.StatusNotImplemented, map[string]string{"error": "search is not configured"})
	}
	items, err := searcher.SearchContent(c.Request().Context(), tenantID, query)
	if err != nil {
		return jsonError(c, http.StatusInternalServerError, "Unable to search content")
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items, "query": query})
}

func (h *ContentHandler) Versions(c echo.Context) error {
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "tenant context required"})
	}
	documentID := strings.TrimSpace(c.Param("id"))
	if documentID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "document id is required"})
	}
	if err := h.ensureContentOwnership(c, documentID); err != nil {
		return jsonError(c, http.StatusForbidden, "Access denied")
	}
	lister, ok := h.Content.Store.(versionLister)
	if !ok {
		return c.JSON(http.StatusNotImplemented, map[string]string{"error": "version history is not configured"})
	}
	versions, err := lister.ListDocumentVersions(c.Request().Context(), tenantID, documentID)
	if err != nil {
		return jsonError(c, http.StatusInternalServerError, "Unable to list version history")
	}
	return c.JSON(http.StatusOK, map[string]any{
		"document_id": documentID,
		"versions":    versions,
	})
}

func (h *ContentHandler) GenerateShare(c echo.Context) error {
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "tenant context required"})
	}
	documentID := strings.TrimSpace(c.Param("id"))
	if documentID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "document id is required"})
	}
	if err := h.ensureContentOwnership(c, documentID); err != nil {
		return jsonError(c, http.StatusForbidden, "Access denied")
	}
	var req generateShareRequest
	if err := c.Bind(&req); err != nil {
		req.ExpiryHours = 72
	}
	if req.ExpiryHours <= 0 || req.ExpiryHours > 72 {
		req.ExpiryHours = 72
	}
	now := time.Now().UTC()
	share := h.Content.GenerateShareLink(documentID, tenantID, "http://localhost:8080/v1/content/share", now)
	share.ExpiresAt = now.Add(time.Duration(req.ExpiryHours) * time.Hour)
	store, ok := h.Content.Store.(shareStore)
	if ok {
		_ = store.SaveShareLink(c.Request().Context(), tenantID, documentID, share.Token, share.ExpiresAt, now)
	}
	return c.JSON(http.StatusOK, map[string]any{
		"url":        fmt.Sprintf("http://localhost:8080/v1/content/share/%s", share.Token),
		"token":      share.Token,
		"expires_at": share.ExpiresAt.Format(time.RFC3339),
	})
}

func (h *ContentHandler) DownloadShared(c echo.Context) error {
	token := strings.TrimSpace(c.Param("token"))
	if token == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "share token is required"})
	}
	share, err := h.Content.ValidateShareToken(token)
	if err != nil {
		return jsonError(c, http.StatusForbidden, "Share link expired or invalid")
	}
	store, ok := h.Content.Store.(shareStore)
	if !ok {
		return jsonError(c, http.StatusNotImplemented, "Share downloads are not configured")
	}
	record, err := store.GetShareLinkByToken(c.Request().Context(), token)
	if err == nil && time.Now().UTC().After(record.ExpiresAt) {
		return jsonError(c, http.StatusForbidden, "Share link expired or invalid")
	}
	file, err := store.GetLatestUploadedFileForDocument(c.Request().Context(), share.TenantID, share.DocumentID)
	if err != nil {
		return jsonError(c, http.StatusNotFound, "Shared file not found")
	}
	content, err := os.ReadFile(file.FilePath)
	if err != nil {
		return jsonError(c, http.StatusNotFound, "Shared file not found")
	}
	watermark := fmt.Sprintf("Confidential | downloader=share-link | timestamp=%s", time.Now().UTC().Format(time.RFC3339))
	c.ResponseWriter().Header().Set("X-Watermark", watermark)
	c.ResponseWriter().Header().Set("X-Content-Type-Options", "nosniff")
	content = append(content, []byte("\n"+watermark)...)
	_ = store.IncrementShareDownload(c.Request().Context(), token, time.Now().UTC())
	c.ResponseWriter().Header().Set("Content-Type", file.MimeType)
	c.ResponseWriter().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", file.FileName))
	c.ResponseWriter().WriteHeader(http.StatusOK)
	_, _ = c.ResponseWriter().Write(content)
	return nil
}

func (h *ContentHandler) ensureContentOwnership(c echo.Context, documentID string) error {
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok {
		return errors.New("tenant context required")
	}
	actorUserID, actorRole := actorIdentity(c)
	if actorRole == "admin" {
		return nil
	}
	items, err := h.Content.Store.ListContentItems(c.Request().Context(), tenantID)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.ID == documentID {
			if strings.TrimSpace(item.CreatedByUserID) != "" && item.CreatedByUserID == actorUserID {
				return nil
			}
			return errors.New("content ownership mismatch")
		}
	}
	return errors.New("content not found")
}

func (h *ContentHandler) ensureOrCreateContentOwnership(c echo.Context, documentID, fileName string) error {
	if err := h.ensureContentOwnership(c, documentID); err == nil {
		return nil
	}
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok {
		return errors.New("tenant context required")
	}
	actorUserID, actorRole := actorIdentity(c)
	if actorRole == "admin" || actorRole == "coordinator" || actorRole == "instructor" {
		return h.Content.SaveContentItem(c.Request().Context(), model.ContentItem{
			ID:              documentID,
			TenantID:        tenantID,
			CreatedByUserID: actorUserID,
			Title:           fileName,
			Difficulty:      1,
			DurationMinutes: 5,
			Version:         1,
		})
	}
	return errors.New("content ownership mismatch")
}
