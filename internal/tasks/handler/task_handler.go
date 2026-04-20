package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/luissebastian953/stratix-core/internal/domain"
	"github.com/luissebastian953/stratix-core/internal/middleware"
	"github.com/luissebastian953/stratix-core/internal/tasks/service"
	"github.com/luissebastian953/stratix-core/pkg/apperror"
	"github.com/luissebastian953/stratix-core/pkg/pagination"
)

type TaskHandler struct {
	svc *service.TaskService
}

func NewTaskHandler(svc *service.TaskService) *TaskHandler {
	return &TaskHandler{svc: svc}
}

func (h *TaskHandler) Register(g *echo.Group) {
	g.GET("", h.list)
	g.POST("", h.create)
	g.GET("/:id", h.getByID)
	g.PATCH("/:id", h.update)
	g.DELETE("/:id", h.delete)
	g.POST("/:id/complete", h.complete)
	g.POST("/:id/archive", h.archive)
	g.GET("/:id/children", h.listChildren)
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (h *TaskHandler) list(c echo.Context) error {
	userID, err := userIDFromCtx(c)
	if err != nil {
		return err
	}

	filter := domain.TaskFilter{
		Types:     parseTypeSlice(c.QueryParam("types")),
		Statuses:  parseStatusSlice(c.QueryParam("statuses")),
		Labels:    parseStringSlice(c.QueryParam("labels")),
		From:      parseTime(c.QueryParam("from")),
		To:        parseTime(c.QueryParam("to")),
		OnlyRoots: c.QueryParam("only_roots") == "true",
		Search:    c.QueryParam("search"),
	}

	if raw := c.QueryParam("parent_id"); raw != "" {
		if id, err := uuid.Parse(raw); err == nil {
			filter.ParentID = &id
		}
	}
	if raw := c.QueryParam("archived"); raw != "" {
		b := raw == "true"
		filter.Archived = &b
	}
	if raw := c.QueryParam("pinned"); raw != "" {
		b := raw == "true"
		filter.Pinned = &b
	}

	p := pagination.Parse(c.QueryParam("limit"), pagination.DecodeCursor(c.QueryParam("cursor")))

	page, err := h.svc.List(c.Request().Context(), userID, filter, p)
	if err != nil {
		return mapError(err)
	}

	return c.JSON(http.StatusOK, toTaskListResponse(page))
}

func (h *TaskHandler) create(c echo.Context) error {
	userID, err := userIDFromCtx(c)
	if err != nil {
		return err
	}

	var req CreateTaskRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body")
	}
	if err := req.Validate(); err != nil {
		return apperror.BadRequest(err.Error())
	}

	task, err := h.svc.Create(c.Request().Context(), userID, req.toServiceInput())
	if err != nil {
		return mapError(err)
	}

	return c.JSON(http.StatusCreated, toTaskResponse(task))
}

func (h *TaskHandler) getByID(c echo.Context) error {
	userID, err := userIDFromCtx(c)
	if err != nil {
		return err
	}

	id, err := parseID(c)
	if err != nil {
		return err
	}

	task, err := h.svc.GetByID(c.Request().Context(), id, userID)
	if err != nil {
		return mapError(err)
	}

	return c.JSON(http.StatusOK, toTaskResponse(task))
}

func (h *TaskHandler) update(c echo.Context) error {
	userID, err := userIDFromCtx(c)
	if err != nil {
		return err
	}

	id, err := parseID(c)
	if err != nil {
		return err
	}

	var req UpdateTaskRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body")
	}
	if err := req.Validate(); err != nil {
		return apperror.BadRequest(err.Error())
	}

	task, err := h.svc.Update(c.Request().Context(), id, userID, req.toServiceInput())
	if err != nil {
		return mapError(err)
	}

	return c.JSON(http.StatusOK, toTaskResponse(task))
}

func (h *TaskHandler) delete(c echo.Context) error {
	userID, err := userIDFromCtx(c)
	if err != nil {
		return err
	}

	id, err := parseID(c)
	if err != nil {
		return err
	}

	if err := h.svc.Delete(c.Request().Context(), id, userID); err != nil {
		return mapError(err)
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *TaskHandler) complete(c echo.Context) error {
	userID, err := userIDFromCtx(c)
	if err != nil {
		return err
	}

	id, err := parseID(c)
	if err != nil {
		return err
	}

	task, err := h.svc.Complete(c.Request().Context(), id, userID)
	if err != nil {
		return mapError(err)
	}

	return c.JSON(http.StatusOK, toTaskResponse(task))
}

func (h *TaskHandler) archive(c echo.Context) error {
	userID, err := userIDFromCtx(c)
	if err != nil {
		return err
	}

	id, err := parseID(c)
	if err != nil {
		return err
	}

	task, err := h.svc.Archive(c.Request().Context(), id, userID)
	if err != nil {
		return mapError(err)
	}

	return c.JSON(http.StatusOK, toTaskResponse(task))
}

func (h *TaskHandler) listChildren(c echo.Context) error {
	userID, err := userIDFromCtx(c)
	if err != nil {
		return err
	}

	id, err := parseID(c)
	if err != nil {
		return err
	}

	tasks, err := h.svc.ListChildren(c.Request().Context(), id, userID)
	if err != nil {
		return mapError(err)
	}

	items := make([]TaskResponse, len(tasks))
	for i, t := range tasks {
		items[i] = toTaskResponse(t)
	}

	return c.JSON(http.StatusOK, items)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func userIDFromCtx(c echo.Context) (uuid.UUID, error) {
	raw, ok := c.Get(middleware.UserIDKey).(string)
	if !ok || raw == "" {
		return uuid.Nil, apperror.Unauthorized("missing user identity")
	}

	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, apperror.Unauthorized("invalid user identity")
	}

	return id, nil
}

func parseID(c echo.Context) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return uuid.Nil, apperror.BadRequest("invalid task id")
	}
	return id, nil
}

func mapError(err error) *apperror.AppError {
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return apperror.Internal(err)
}

// parseStringSlice splits a comma-separated query param into a string slice.
// Returns nil (not empty slice) when the param is absent so callers can
// distinguish "not provided" from "provided as empty".
func parseStringSlice(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func parseTypeSlice(raw string) []domain.TaskType {
	strs := parseStringSlice(raw)
	if strs == nil {
		return nil
	}
	out := make([]domain.TaskType, len(strs))
	for i, s := range strs {
		out[i] = domain.TaskType(s)
	}
	return out
}

func parseStatusSlice(raw string) []domain.Status {
	strs := parseStringSlice(raw)
	if strs == nil {
		return nil
	}
	out := make([]domain.Status, len(strs))
	for i, s := range strs {
		out[i] = domain.Status(s)
	}
	return out
}

func parseTime(raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil
	}
	utc := t.UTC()
	return &utc
}

