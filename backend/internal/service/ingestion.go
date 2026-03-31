package service

import (
	"context"
	"errors"
	"math/rand"
	"strings"
	"sync"
	"time"

	"trainingops/internal/model"
	"trainingops/internal/security"
)

type ProxyPoolManager struct {
	mu      sync.Mutex
	proxies []string
	index   int
}

func NewProxyPoolManager(proxies []string) *ProxyPoolManager {
	return &ProxyPoolManager{proxies: append([]string(nil), proxies...)}
}

func (m *ProxyPoolManager) Next() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.proxies) == 0 {
		return "direct"
	}
	proxy := m.proxies[m.index%len(m.proxies)]
	m.index++
	return proxy
}

type UserAgentRotator struct {
	mu         sync.Mutex
	userAgents []string
	index      int
}

func NewUserAgentRotator(userAgents []string) *UserAgentRotator {
	return &UserAgentRotator{userAgents: append([]string(nil), userAgents...)}
}

func (r *UserAgentRotator) Next() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.userAgents) == 0 {
		return "TrainingOpsBot/1.0"
	}
	ua := r.userAgents[r.index%len(r.userAgents)]
	r.index++
	return ua
}

type ScraperState string

const (
	ScraperStateQueued       ScraperState = "queued"
	ScraperStateRunning      ScraperState = "running"
	ScraperStateManualReview ScraperState = "manual_review"
	ScraperStateCompleted    ScraperState = "completed"
	ScraperStateFailed       ScraperState = "failed"
)

type ScraperJob struct {
	ID        string
	URL       string
	Proxy     string
	UserAgent string
	Delay     time.Duration
	State     ScraperState
	Reason    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type IngestionBot struct {
	ProxyPool *ProxyPoolManager
	UARotator *UserAgentRotator
	Rand      *rand.Rand
	Store     IngestionStore

	mu         sync.Mutex
	tenantHits map[string][]time.Time
}

type IngestionStore interface {
	UpsertIngestionSession(ctx context.Context, session model.IngestionSession) (*model.IngestionSession, error)
	SaveIngestionJob(ctx context.Context, tenantID, sessionID string, job ScraperJob) (*ScraperJob, error)
	EnqueueManualReview(ctx context.Context, tenantID, jobID, reason string) error
}

var ErrIngestionRateLimited = errors.New("ingestion rate limit exceeded")

func NewIngestionBot(proxyPool *ProxyPoolManager, rotator *UserAgentRotator, seed int64, store IngestionStore) *IngestionBot {
	return &IngestionBot{
		ProxyPool:  proxyPool,
		UARotator:  rotator,
		Rand:       rand.New(rand.NewSource(seed)),
		Store:      store,
		tenantHits: map[string][]time.Time{},
	}
}

func (b *IngestionBot) PrepareJob(url string, now time.Time) ScraperJob {
	delay := time.Duration(2+b.Rand.Intn(13)) * time.Second
	return ScraperJob{
		ID:        security.GenerateUUID(),
		URL:       url,
		Proxy:     b.ProxyPool.Next(),
		UserAgent: b.UARotator.Next(),
		Delay:     delay,
		State:     ScraperStateQueued,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (b *IngestionBot) DetectCaptcha(body string) bool {
	signatures := []string{"captcha", "manual review", "verify you are human", "bot detection"}
	normalized := strings.ToLower(body)
	for _, sig := range signatures {
		if strings.Contains(normalized, sig) {
			return true
		}
	}
	return false
}

func (b *IngestionBot) Run(job ScraperJob, body string, now time.Time) ScraperJob {
	job.State = ScraperStateRunning
	job.UpdatedAt = now
	if b.DetectCaptcha(body) {
		job.State = ScraperStateManualReview
		job.Reason = "captcha detected"
		job.UpdatedAt = now
		return job
	}
	job.State = ScraperStateCompleted
	job.Reason = "ingestion complete"
	job.UpdatedAt = now
	return job
}

func (b *IngestionBot) RunPersistent(ctx context.Context, tenantID, actorUserID, url, body string, now time.Time) (ScraperJob, error) {
	if b.Store == nil {
		return ScraperJob{}, errors.New("ingestion store is not configured")
	}
	if err := b.allowRequest(tenantID, now); err != nil {
		return ScraperJob{}, err
	}

	job := b.PrepareJob(url, now)
	session, err := b.Store.UpsertIngestionSession(ctx, model.IngestionSession{
		ID:           security.GenerateUUID(),
		TenantID:     tenantID,
		ActorUserID:  actorUserID,
		Proxy:        job.Proxy,
		UserAgent:    job.UserAgent,
		RequestCount: len(b.tenantHits[tenantID]),
		LastSeenAt:   now,
	})
	if err != nil {
		return ScraperJob{}, err
	}

	runAt := now.Add(job.Delay)
	result := b.Run(job, body, runAt)
	saved, err := b.Store.SaveIngestionJob(ctx, tenantID, session.ID, result)
	if err != nil {
		return ScraperJob{}, err
	}
	if saved.State == ScraperStateManualReview {
		if err := b.Store.EnqueueManualReview(ctx, tenantID, saved.ID, saved.Reason); err != nil {
			return ScraperJob{}, err
		}
	}
	return *saved, nil
}

func (b *IngestionBot) allowRequest(tenantID string, now time.Time) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	windowStart := now.Add(-1 * time.Minute)
	recent := b.tenantHits[tenantID][:0]
	for _, hit := range b.tenantHits[tenantID] {
		if hit.After(windowStart) {
			recent = append(recent, hit)
		}
	}
	if len(recent) >= 5 {
		b.tenantHits[tenantID] = recent
		return ErrIngestionRateLimited
	}
	recent = append(recent, now)
	b.tenantHits[tenantID] = recent
	return nil
}
