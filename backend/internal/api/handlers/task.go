package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"trainingops/internal/model"
	"trainingops/internal/repository"
	"trainingops/internal/service"
	"trainingops/internal/tenant"
)

type TaskHandler struct {
	Tasks *service.TaskService
	Store repository.TaskStore
}

type createTaskRequest struct {
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	DueDate          string   `json:"due_date"`
	DependencyIDs    []string `json:"dependency_ids"`
	EstimatedMinutes int      `json:"estimated_minutes"`
	ActualMinutes    int      `json:"actual_minutes"`
}

type updateTaskRequest struct {
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	DueDate          string   `json:"due_date"`
	DependencyIDs    []string `json:"dependency_ids"`
	EstimatedMinutes int      `json:"estimated_minutes"`
	ActualMinutes    int      `json:"actual_minutes"`
	ExpectedVersion  int64    `json:"expected_version"`
}

func (h *TaskHandler) Create(c echo.Context) error {
	var req createTaskRequest
	if err := c.Bind(&req); err != nil {
		return jsonError(c, http.StatusBadRequest, "invalid request body")
	}
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok {
		return jsonError(c, http.StatusUnauthorized, "tenant context required")
	}
	task := model.Task{
		ID:               newUUIDString(),
		TenantID:         tenantID,
		MilestoneID:      c.Param("id"),
		Title:            req.Title,
		Description:      req.Description,
		DependencyIDs:    req.DependencyIDs,
		EstimatedMinutes: req.EstimatedMinutes,
		ActualMinutes:    req.ActualMinutes,
		Version:          1,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	if dueDate, err := parseTaskDueDate(req.DueDate); err != nil {
		return jsonError(c, http.StatusBadRequest, "due_date must be YYYY-MM-DD")
	} else {
		task.DueDate = dueDate
	}
	existing, _ := h.Store.ListTasks(c.Request().Context(), tenantID)
	existing = append(existing, task)
	if err := h.Tasks.ValidateDAG(existing); err != nil {
		return jsonError(c, http.StatusConflict, "Circular dependency detected")
	}
	if err := h.Store.UpsertTask(c.Request().Context(), task); err != nil {
		return jsonError(c, http.StatusInternalServerError, "Unable to create task")
	}
	return c.JSON(http.StatusCreated, task)
}

func (h *TaskHandler) ListByMilestone(c echo.Context) error {
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok {
		return jsonError(c, http.StatusUnauthorized, "tenant context required")
	}
	milestoneID := c.Param("id")
	tasks, err := h.Store.ListTasks(c.Request().Context(), tenantID)
	if err != nil {
		return jsonError(c, http.StatusInternalServerError, "Unable to list tasks")
	}
	filtered := []model.Task{}
	for _, task := range tasks {
		if task.MilestoneID == milestoneID {
			filtered = append(filtered, task)
		}
	}
	return c.JSON(http.StatusOK, map[string]any{"tasks": filtered})
}

func (h *TaskHandler) Update(c echo.Context) error {
	var req updateTaskRequest
	if err := c.Bind(&req); err != nil {
		return jsonError(c, http.StatusBadRequest, "invalid request body")
	}
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok {
		return jsonError(c, http.StatusUnauthorized, "tenant context required")
	}
	current, err := h.Store.GetTask(c.Request().Context(), tenantID, c.Param("id"))
	if err != nil {
		return jsonError(c, http.StatusNotFound, "Task not found")
	}
	current.Title = req.Title
	current.Description = req.Description
	if dueDate, err := parseTaskDueDate(req.DueDate); err != nil {
		return jsonError(c, http.StatusBadRequest, "due_date must be YYYY-MM-DD")
	} else {
		current.DueDate = dueDate
	}
	current.DependencyIDs = req.DependencyIDs
	current.EstimatedMinutes = req.EstimatedMinutes
	current.ActualMinutes = req.ActualMinutes
	current.UpdatedAt = time.Now().UTC()
	if err := h.Tasks.UpdateTask(c.Request().Context(), *current, req.ExpectedVersion); err != nil {
		if err == service.ErrTaskVersionConflict {
			return jsonError(c, http.StatusConflict, "Task was updated by another user")
		}
		return jsonError(c, http.StatusConflict, "Task dependency validation failed")
	}
	return c.JSON(http.StatusOK, current)
}

func parseTaskDueDate(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, err
	}
	due := parsed.UTC()
	return &due, nil
}

func (h *TaskHandler) Delete(c echo.Context) error {
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok {
		return jsonError(c, http.StatusUnauthorized, "tenant context required")
	}
	if err := h.Store.DeleteTask(c.Request().Context(), tenantID, c.Param("id")); err != nil {
		return jsonError(c, http.StatusInternalServerError, "Unable to delete task")
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *TaskHandler) AddDependencies(c echo.Context) error {
	var req struct {
		DependencyIDs []string `json:"dependency_ids"`
	}
	if err := c.Bind(&req); err != nil {
		return jsonError(c, http.StatusBadRequest, "invalid request body")
	}
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok {
		return jsonError(c, http.StatusUnauthorized, "tenant context required")
	}
	task, err := h.Store.GetTask(c.Request().Context(), tenantID, c.Param("id"))
	if err != nil {
		return jsonError(c, http.StatusNotFound, "Task not found")
	}
	task.DependencyIDs = req.DependencyIDs
	all, _ := h.Store.ListTasks(c.Request().Context(), tenantID)
	for i := range all {
		if all[i].ID == task.ID {
			all[i] = *task
		}
	}
	if err := h.Tasks.ValidateDAG(all); err != nil {
		return jsonError(c, http.StatusConflict, "Circular dependency detected")
	}
	task.UpdatedAt = time.Now().UTC()
	task.Version++
	if err := h.Store.UpsertTask(c.Request().Context(), *task); err != nil {
		return jsonError(c, http.StatusInternalServerError, "Unable to update task dependencies")
	}
	return c.JSON(http.StatusOK, task)
}
