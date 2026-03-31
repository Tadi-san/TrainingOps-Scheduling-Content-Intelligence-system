package memory

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"

	"trainingops/internal/model"
)

var ErrContentNotFound = errors.New("content not found")

type ContentRepository struct {
	mu        sync.Mutex
	items     map[string]model.ContentItem
	tags      map[string]model.ContentTag
	documents map[string]model.Document
	versions  map[string][]model.DocumentVersion
	uploads   map[string]model.UploadSession
}

func NewContentRepository() *ContentRepository {
	return &ContentRepository{
		items:     map[string]model.ContentItem{},
		tags:      map[string]model.ContentTag{},
		documents: map[string]model.Document{},
		versions:  map[string][]model.DocumentVersion{},
		uploads:   map[string]model.UploadSession{},
	}
}

func contentKey(tenantID, id string) string { return tenantID + ":" + id }

func (r *ContentRepository) UpsertContentItem(ctx context.Context, item model.ContentItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[contentKey(item.TenantID, item.ID)] = item
	return nil
}

func (r *ContentRepository) DeleteContentItem(ctx context.Context, tenantID, itemID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.items, contentKey(tenantID, itemID))
	return nil
}

func (r *ContentRepository) ListContentItems(ctx context.Context, tenantID string) ([]model.ContentItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []model.ContentItem
	for _, item := range r.items {
		if item.TenantID == tenantID {
			out = append(out, item)
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Title) < strings.ToLower(out[j].Title) })
	return out, nil
}

func (r *ContentRepository) UpsertTag(ctx context.Context, tag model.ContentTag) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tags[contentKey(tag.TenantID, tag.ID)] = tag
	return nil
}

func (r *ContentRepository) DeleteTag(ctx context.Context, tenantID, tagID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tags, contentKey(tenantID, tagID))
	return nil
}

func (r *ContentRepository) ListTags(ctx context.Context, tenantID string) ([]model.ContentTag, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []model.ContentTag
	for _, tag := range r.tags {
		if tag.TenantID == tenantID {
			out = append(out, tag)
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out, nil
}

func (r *ContentRepository) SaveDocument(ctx context.Context, document model.Document) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.documents[contentKey(document.TenantID, document.ID)] = document
	return nil
}

func (r *ContentRepository) ListDocuments(ctx context.Context, tenantID string) ([]model.Document, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []model.Document
	for _, document := range r.documents {
		if document.TenantID == tenantID {
			out = append(out, document)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Title < out[j].Title })
	return out, nil
}

func (r *ContentRepository) AddDocumentVersion(ctx context.Context, version model.DocumentVersion) (model.DocumentVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := contentKey(version.TenantID, version.DocumentID)
	version.Version = len(r.versions[key]) + 1
	r.versions[key] = append(r.versions[key], version)
	return version, nil
}

func (r *ContentRepository) ListDocumentVersions(ctx context.Context, tenantID, documentID string) ([]model.DocumentVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := contentKey(tenantID, documentID)
	out := make([]model.DocumentVersion, len(r.versions[key]))
	copy(out, r.versions[key])
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

func (r *ContentRepository) SaveUploadSession(ctx context.Context, session model.UploadSession) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.uploads[contentKey(session.TenantID, session.ID)] = session
	return nil
}

func (r *ContentRepository) GetUploadSession(ctx context.Context, tenantID, sessionID string) (*model.UploadSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	session, ok := r.uploads[contentKey(tenantID, sessionID)]
	if !ok {
		return nil, ErrContentNotFound
	}
	copy := session
	return &copy, nil
}

func (r *ContentRepository) DeleteUploadSession(ctx context.Context, tenantID, sessionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.uploads, contentKey(tenantID, sessionID))
	return nil
}
