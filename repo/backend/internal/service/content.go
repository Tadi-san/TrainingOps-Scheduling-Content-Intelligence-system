package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"trainingops/internal/model"
	"trainingops/internal/repository"
)

var (
	ErrInvalidContentMetadata = errors.New("difficulty must be 1-5 and duration must be 5-480 minutes")
	ErrUploadChunkChecksum    = errors.New("chunk checksum validation failed")
	ErrUploadChecksumMismatch = errors.New("upload checksum mismatch")
	ErrUploadIncomplete       = errors.New("upload session is incomplete")
)

type ContentService struct {
	Store       repository.ContentStore
	ShareSecret []byte
	StoragePath string
}

func NewContentService(store repository.ContentStore, shareSecret []byte, storagePath string) *ContentService {
	return &ContentService{Store: store, ShareSecret: shareSecret, StoragePath: storagePath}
}

func ValidateContentMetadata(difficulty, durationMinutes int) error {
	if difficulty < 1 || difficulty > 5 {
		return ErrInvalidContentMetadata
	}
	if durationMinutes < 5 || durationMinutes > 480 {
		return ErrInvalidContentMetadata
	}
	return nil
}

func (s *ContentService) SaveContentItem(ctx context.Context, item model.ContentItem) error {
	duration := item.DurationMinutes
	if duration == 0 {
		duration = item.DurationM
	}
	if err := ValidateContentMetadata(item.Difficulty, duration); err != nil {
		return err
	}
	item.DurationMinutes = duration
	if item.Version == 0 {
		item.Version = 1
	}
	return s.Store.UpsertContentItem(ctx, item)
}

func (s *ContentService) StartUpload(ctx context.Context, session model.UploadSession) error {
	if !allowedFile(session.FileName) {
		return errors.New("unsupported file type")
	}
	if session.ReceivedChunks == nil {
		session.ReceivedChunks = map[int][]byte{}
	}
	session.CreatedAt = time.Now().UTC()
	session.UpdatedAt = session.CreatedAt
	return s.Store.SaveUploadSession(ctx, session)
}

func (s *ContentService) AppendChunk(ctx context.Context, tenantID, sessionID string, index int, chunk []byte, checksum string) (bool, error) {
	session, err := s.Store.GetUploadSession(ctx, tenantID, sessionID)
	if err != nil {
		return false, err
	}

	sum := sha256.Sum256(chunk)
	if checksum != fmt.Sprintf("%x", sum[:]) {
		return false, ErrUploadChunkChecksum
	}

	if session.ReceivedChunks == nil {
		session.ReceivedChunks = map[int][]byte{}
	}
	session.ReceivedChunks[index] = append([]byte(nil), chunk...)
	session.UpdatedAt = time.Now().UTC()
	if err := s.Store.SaveUploadSession(ctx, *session); err != nil {
		return false, err
	}

	return len(session.ReceivedChunks) >= session.ExpectedChunks, nil
}

func (s *ContentService) FinalizeUpload(ctx context.Context, tenantID, sessionID string, now time.Time) (*model.DocumentVersion, error) {
	session, err := s.Store.GetUploadSession(ctx, tenantID, sessionID)
	if err != nil {
		return nil, err
	}
	if len(session.ReceivedChunks) < session.ExpectedChunks {
		return nil, ErrUploadIncomplete
	}

	ordered := make([]int, 0, len(session.ReceivedChunks))
	for idx := range session.ReceivedChunks {
		ordered = append(ordered, idx)
	}
	sort.Ints(ordered)

	hasher := sha256.New()
	for _, idx := range ordered {
		hasher.Write(session.ReceivedChunks[idx])
	}
	actualChecksum := fmt.Sprintf("%x", hasher.Sum(nil))
	if session.ExpectedChecksum != "" && !strings.EqualFold(session.ExpectedChecksum, actualChecksum) {
		return nil, ErrUploadChecksumMismatch
	}

	version := model.DocumentVersion{
		ID:         fmt.Sprintf("dv_%d", now.UnixNano()),
		TenantID:   tenantID,
		DocumentID: session.DocumentID,
		Version:    0,
		FileName:   session.FileName,
		Checksum:   actualChecksum,
		SizeBytes:  int64(totalSize(session.ReceivedChunks)),
		CreatedAt:  now,
	}
	if err := s.writeUploadToDisk(session, ordered); err != nil {
		return nil, err
	}
	storedVersion, err := s.Store.AddDocumentVersion(ctx, version)
	if err != nil {
		return nil, err
	}
	if err := s.Store.DeleteUploadSession(ctx, tenantID, sessionID); err != nil {
		return nil, err
	}
	return &storedVersion, nil
}

func (s *ContentService) GenerateShareLink(documentID, tenantID, baseURL string, now time.Time) model.ShareLink {
	expiresAt := now.Add(72 * time.Hour)
	payload := tenantID + "|" + documentID + "|" + strconv.FormatInt(expiresAt.Unix(), 10)
	mac := hmac.New(sha256.New, s.ShareSecret)
	mac.Write([]byte(payload))
	token := base64.RawURLEncoding.EncodeToString([]byte(payload + "|" + fmt.Sprintf("%x", mac.Sum(nil))))
	linkURL := strings.TrimRight(baseURL, "/") + "/share?token=" + token
	return model.ShareLink{
		URL:        linkURL,
		Token:      token,
		DocumentID: documentID,
		TenantID:   tenantID,
		ExpiresAt:  expiresAt,
	}
}

func (s *ContentService) ValidateShareToken(token string) (model.ShareLink, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return model.ShareLink{}, err
	}
	parts := strings.Split(string(decoded), "|")
	if len(parts) != 4 {
		return model.ShareLink{}, errors.New("invalid share token")
	}

	tenantID, documentID, expiryUnix, sig := parts[0], parts[1], parts[2], parts[3]
	payload := tenantID + "|" + documentID + "|" + expiryUnix
	mac := hmac.New(sha256.New, s.ShareSecret)
	mac.Write([]byte(payload))
	expected := fmt.Sprintf("%x", mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return model.ShareLink{}, errors.New("invalid share token signature")
	}

	expiry, err := strconv.ParseInt(expiryUnix, 10, 64)
	if err != nil {
		return model.ShareLink{}, err
	}
	expiresAt := time.Unix(expiry, 0).UTC()
	if time.Now().UTC().After(expiresAt) {
		return model.ShareLink{}, errors.New("share token expired")
	}
	return model.ShareLink{
		Token:      token,
		DocumentID: documentID,
		TenantID:   tenantID,
		ExpiresAt:  expiresAt,
	}, nil
}

func (s *ContentService) FindDuplicateTags(ctx context.Context, tenantID string) (map[string][]model.ContentTag, error) {
	tags, err := s.Store.ListTags(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	grouped := map[string][]model.ContentTag{}
	for _, tag := range tags {
		key := normalizeContentKey(tag.Name)
		grouped[key] = append(grouped[key], tag)
	}
	for key, values := range grouped {
		if len(values) < 2 {
			delete(grouped, key)
		}
	}
	return grouped, nil
}

func (s *ContentService) MergeDuplicateTags(ctx context.Context, tenantID string) (int, error) {
	duplicates, err := s.FindDuplicateTags(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	merged := 0
	for _, group := range duplicates {
		for i := 1; i < len(group); i++ {
			if err := s.Store.DeleteTag(ctx, tenantID, group[i].ID); err != nil {
				return merged, err
			}
			merged++
		}
	}
	return merged, nil
}

func (s *ContentService) FindDuplicateItems(ctx context.Context, tenantID string) (map[string][]model.ContentItem, error) {
	items, err := s.Store.ListContentItems(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	grouped := map[string][]model.ContentItem{}
	for _, item := range items {
		key := normalizeContentKey(item.Title + "|" + item.CategoryID)
		grouped[key] = append(grouped[key], item)
	}
	for key, values := range grouped {
		if len(values) < 2 {
			delete(grouped, key)
		}
	}
	return grouped, nil
}

func (s *ContentService) MergeDuplicateItems(ctx context.Context, tenantID string) (int, error) {
	duplicates, err := s.FindDuplicateItems(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	merged := 0
	for _, group := range duplicates {
		for i := 1; i < len(group); i++ {
			if err := s.Store.DeleteContentItem(ctx, tenantID, group[i].ID); err != nil {
				return merged, err
			}
			merged++
		}
	}
	return merged, nil
}

func normalizeContentKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func allowedFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".pdf", ".jpg", ".jpeg", ".png", ".txt":
		return true
	default:
		return false
	}
}

func totalSize(chunks map[int][]byte) int {
	total := 0
	for _, chunk := range chunks {
		total += len(chunk)
	}
	return total
}

func (s *ContentService) writeUploadToDisk(session *model.UploadSession, ordered []int) error {
	if s.StoragePath == "" {
		s.StoragePath = "./uploads"
	}
	if err := os.MkdirAll(s.StoragePath, 0o755); err != nil {
		return err
	}
	path := filepath.Join(s.StoragePath, fmt.Sprintf("%s_%s", session.DocumentID, filepath.Base(session.FileName)))
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	for _, idx := range ordered {
		if _, err := file.Write(session.ReceivedChunks[idx]); err != nil {
			return err
		}
	}
	return nil
}
