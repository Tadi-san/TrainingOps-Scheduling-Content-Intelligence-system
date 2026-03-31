package service

import (
	"testing"

	"trainingops/internal/model"
)

func TestValidateDAG_DetectsCycle(t *testing.T) {
	svc := NewTaskService(nil)
	tasks := []model.Task{
		{ID: "a", DependencyIDs: []string{"b"}},
		{ID: "b", DependencyIDs: []string{"c"}},
		{ID: "c", DependencyIDs: []string{"a"}},
	}

	if err := svc.ValidateDAG(tasks); err == nil {
		t.Fatal("expected cycle detection error")
	}
}

func TestCalculateEffort_SumsEstimatedAndActual(t *testing.T) {
	tasks := []model.Task{
		{EstimatedMinutes: 30, ActualMinutes: 45},
		{EstimatedMinutes: 60, ActualMinutes: 50},
	}

	summary := CalculateEffort(tasks)
	if summary.EstimatedMinutes != 90 {
		t.Fatalf("expected estimated 90, got %d", summary.EstimatedMinutes)
	}
	if summary.ActualMinutes != 95 {
		t.Fatalf("expected actual 95, got %d", summary.ActualMinutes)
	}
	if summary.VarianceMinutes != 5 {
		t.Fatalf("expected variance 5, got %d", summary.VarianceMinutes)
	}
}
