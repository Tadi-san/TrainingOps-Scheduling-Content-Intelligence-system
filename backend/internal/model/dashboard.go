package model

import "time"

type KPI struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Delta string `json:"delta"`
}

type HeatmapCell struct {
	Day   string `json:"day"`
	Hour  int    `json:"hour"`
	Load  int    `json:"load"`
	State string `json:"state"`
}

type CalendarSession struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	StartsAt time.Time `json:"startsAt"`
	EndsAt   time.Time `json:"endsAt"`
	Room     string    `json:"room"`
	Owner    string    `json:"owner"`
	Status   string    `json:"status"`
}

type DashboardData struct {
	Role            string            `json:"role"`
	Title           string            `json:"title"`
	Subtitle        string            `json:"subtitle"`
	KPIs            []KPI             `json:"kpis"`
	Heatmap         []HeatmapCell     `json:"heatmap"`
	Calendar        []CalendarSession `json:"calendar"`
	CountdownEnd    *time.Time        `json:"countdownEnd,omitempty"`
	TaskOrdering    []string          `json:"taskOrdering"`
	PreviewDocument string            `json:"previewDocument"`
	PreviewImage    string            `json:"previewImage"`
}

type IngestionJob struct {
	ID           string    `json:"id"`
	URL          string    `json:"url"`
	Proxy        string    `json:"proxy"`
	UserAgent    string    `json:"userAgent"`
	DelaySeconds int       `json:"delaySeconds"`
	State        string    `json:"state"`
	Reason       string    `json:"reason"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type ObservabilityEvent struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenantId"`
	Type      string    `json:"type"`
	Level     string    `json:"level"`
	Detail    string    `json:"detail"`
	CreatedAt time.Time `json:"createdAt"`
}

type CohortProfile struct {
	Window      string    `json:"window"`
	Total       int       `json:"total"`
	SuccessRate float64   `json:"successRate"`
	FailureRate float64   `json:"failureRate"`
	TopCategory string    `json:"topCategory"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Anomaly struct {
	Kind       string    `json:"kind"`
	Severity   string    `json:"severity"`
	Message    string    `json:"message"`
	Count      int       `json:"count"`
	Window     string    `json:"window"`
	DetectedAt time.Time `json:"detectedAt"`
}

type Report struct {
	Filename string `json:"filename"`
	MimeType string `json:"mimeType"`
	Body     []byte `json:"body"`
}
