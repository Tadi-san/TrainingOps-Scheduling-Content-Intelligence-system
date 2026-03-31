package memory

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"

	"trainingops/internal/model"
)

var ErrTaskNotFound = errors.New("task not found")

type TaskRepository struct {
	mu    sync.Mutex
	tasks map[string]model.Task
}

func NewTaskRepository() *TaskRepository {
	return &TaskRepository{tasks: map[string]model.Task{}}
}

func taskKey(tenantID, taskID string) string { return tenantID + ":" + taskID }

func (r *TaskRepository) UpsertTask(ctx context.Context, task model.Task) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tasks[taskKey(task.TenantID, task.ID)] = task
	return nil
}

func (r *TaskRepository) GetTask(ctx context.Context, tenantID, taskID string) (*model.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskKey(tenantID, taskID)]
	if !ok {
		return nil, ErrTaskNotFound
	}
	copy := task
	return &copy, nil
}

func (r *TaskRepository) ListTasks(ctx context.Context, tenantID string) ([]model.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []model.Task
	for _, task := range r.tasks {
		if task.TenantID == tenantID {
			out = append(out, task)
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Title) < strings.ToLower(out[j].Title) })
	return out, nil
}

func (r *TaskRepository) DeleteTask(ctx context.Context, tenantID, taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tasks, taskKey(tenantID, taskID))
	return nil
}
