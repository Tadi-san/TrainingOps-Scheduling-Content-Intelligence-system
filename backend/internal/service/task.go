package service

import (
	"context"
	"errors"
	"fmt"

	"trainingops/internal/model"
	"trainingops/internal/repository"
)

var (
	ErrTaskCycleDetected   = errors.New("task dependency cycle detected")
	ErrTaskVersionConflict = errors.New("task version conflict")
)

type TaskService struct {
	Store repository.TaskStore
}

func NewTaskService(store repository.TaskStore) *TaskService {
	return &TaskService{Store: store}
}

func (s *TaskService) ValidateDAG(tasks []model.Task) error {
	graph := map[string][]string{}
	for _, task := range tasks {
		graph[task.ID] = append([]string(nil), task.DependencyIDs...)
	}

	visited := map[string]bool{}
	stack := map[string]bool{}
	var visit func(string) bool
	visit = func(id string) bool {
		if stack[id] {
			return true
		}
		if visited[id] {
			return false
		}
		visited[id] = true
		stack[id] = true
		for _, dep := range graph[id] {
			if visit(dep) {
				return true
			}
		}
		delete(stack, id)
		return false
	}

	for id := range graph {
		if visit(id) {
			return ErrTaskCycleDetected
		}
	}
	return nil
}

func CalculateEffort(tasks []model.Task) model.TaskEffortSummary {
	summary := model.TaskEffortSummary{}
	for _, task := range tasks {
		summary.EstimatedMinutes += task.EstimatedMinutes
		summary.ActualMinutes += task.ActualMinutes
	}
	summary.VarianceMinutes = summary.ActualMinutes - summary.EstimatedMinutes
	return summary
}

func (s *TaskService) UpdateTask(ctx context.Context, updated model.Task, expectedVersion int64) error {
	current, err := s.Store.GetTask(ctx, updated.TenantID, updated.ID)
	if err != nil {
		return err
	}
	if current.Version != expectedVersion {
		return ErrTaskVersionConflict
	}
	updated.Version = expectedVersion + 1
	return s.Store.UpsertTask(ctx, updated)
}

func (s *TaskService) DependencySummary(tasks []model.Task) string {
	if err := s.ValidateDAG(tasks); err != nil {
		return err.Error()
	}
	return fmt.Sprintf("%d tasks validated", len(tasks))
}
