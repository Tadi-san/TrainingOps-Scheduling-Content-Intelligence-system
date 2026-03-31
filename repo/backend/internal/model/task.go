package model

import "time"

type Task struct {
	ID               string
	TenantID         string
	MilestoneID      string
	ParentTaskID     string
	Title            string
	Description      string
	DueDate          *time.Time
	DependencyIDs    []string
	EstimatedMinutes int
	ActualMinutes    int
	Version          int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type TaskEffortSummary struct {
	EstimatedMinutes int
	ActualMinutes    int
	VarianceMinutes  int
}
