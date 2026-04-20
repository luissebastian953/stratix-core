package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/luissebastian953/stratix-core/internal/domain"
	"github.com/luissebastian953/stratix-core/internal/events"
	"github.com/luissebastian953/stratix-core/pkg/apperror"
	"github.com/luissebastian953/stratix-core/pkg/pagination"
	"github.com/luissebastian953/stratix-core/pkg/storage"
)

// TaskService orchestrates all task use cases.
// It owns the application logic between the HTTP handler and the domain
// aggregate — input mapping, ownership checks, event dispatch, storage.
type TaskService struct {
	repo       domain.TaskRepository
	dispatcher *events.Dispatcher
	storage    storage.Client
}

func NewTaskService(
	repo domain.TaskRepository,
	dispatcher *events.Dispatcher,
	storage storage.Client,
) *TaskService {
	return &TaskService{repo: repo, dispatcher: dispatcher, storage: storage}
}

// ── Input types ───────────────────────────────────────────────────────────────

type CreateInput struct {
	Type           domain.TaskType
	Title          string
	Details        string
	Priority       *domain.Priority
	OccurrenceType domain.OccurrenceType
	StartAt        *time.Time
	EndAt          *time.Time
	Location       string
	Labels         []string
	Links          []domain.TaskLink
	ParentID       *uuid.UUID
	RecurrenceRule *domain.RecurrenceRule
}

// UpdateInput carries optional fields — only non-nil values are applied.
type UpdateInput struct {
	Title    *string
	Details  *string
	Status   *domain.Status
	Priority *domain.Priority
	StartAt  *time.Time
	EndAt    *time.Time
	Location *string
	Labels   []string
	Links    []domain.TaskLink
	Pinned   *bool
	Archived *bool
	ParentID *uuid.UUID
}

// ── Use cases ─────────────────────────────────────────────────────────────────

// Create constructs and persists a new task, then dispatches domain events.
func (s *TaskService) Create(ctx context.Context, userID uuid.UUID, in CreateInput) (*domain.Task, error) {
	task, err := new(domain.Task).NewTask(userID, in.Type, in.Title)
	if err != nil {
		return nil, apperror.BadRequest(err.Error())
	}

	if in.Details != "" {
		task.SetDetails(in.Details)
	}
	if in.Priority != nil {
		if err := task.SetPriority(*in.Priority); err != nil {
			return nil, apperror.BadRequest(err.Error())
		}
	}
	if in.Location != "" {
		task.SetLocation(in.Location)
	}
	if len(in.Labels) > 0 {
		task.SetLabels(in.Labels)
	}
	if len(in.Links) > 0 {
		if err := task.SetLinks(in.Links); err != nil {
			return nil, apperror.BadRequest(err.Error())
		}
	}
	if in.ParentID != nil {
		if err := task.SetParent(*in.ParentID); err != nil {
			return nil, apperror.BadRequest(err.Error())
		}
	}

	switch in.OccurrenceType {
	case domain.OccurrenceOnce:
		if in.StartAt != nil {
			if err := task.Schedule(*in.StartAt, in.EndAt); err != nil {
				return nil, apperror.BadRequest(err.Error())
			}
		}
	case domain.OccurrenceRecurring:
		if in.RecurrenceRule == nil {
			return nil, apperror.BadRequest("recurrence_rule is required for recurring tasks")
		}
		if in.StartAt == nil {
			return nil, apperror.BadRequest("start_at is required for recurring tasks")
		}
		if err := task.SetRecurring(*in.RecurrenceRule, *in.StartAt); err != nil {
			return nil, apperror.BadRequest(err.Error())
		}
	case domain.OccurrenceUnbound:
		if in.Type != domain.TaskTypeLog {
			return nil, apperror.BadRequest("unbound occurrence is only valid for log tasks")
		}
	}

	if err := s.repo.Save(ctx, task); err != nil {
		return nil, apperror.Internal(err)
	}

	s.dispatch(ctx, task)
	return task, nil
}

// GetByID retrieves a task, verifying it belongs to the requesting user.
func (s *TaskService) GetByID(ctx context.Context, id, userID uuid.UUID) (*domain.Task, error) {
	task, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if domain.IsNotFound(err) {
			return nil, apperror.NotFound("task")
		}
		return nil, apperror.Internal(err)
	}
	// Return 404 rather than 403 — don't reveal the resource exists.
	if task.UserID != userID {
		return nil, apperror.NotFound("task")
	}
	return task, nil
}

// List returns a paginated slice of tasks for the user matching the filter.
func (s *TaskService) List(
	ctx context.Context,
	userID uuid.UUID,
	filter domain.TaskFilter,
	p pagination.Params,
) (pagination.Page[*domain.Task], error) {
	filter.Limit = p.Limit
	filter.Cursor = p.Cursor

	tasks, err := s.repo.ListByUser(ctx, userID, filter)
	if err != nil {
		return pagination.Page[*domain.Task]{}, apperror.Internal(err)
	}

	return pagination.NewPage(tasks, p.Limit, func(t *domain.Task) string {
		return pagination.NewCursor(t.CreatedAt, t.ID)
	}), nil
}

// Update applies a partial update to a task the user owns.
func (s *TaskService) Update(ctx context.Context, id, userID uuid.UUID, in UpdateInput) (*domain.Task, error) {
	task, err := s.GetByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}

	if in.Title != nil {
		if err := task.SetTitle(*in.Title); err != nil {
			return nil, apperror.BadRequest(err.Error())
		}
	}
	if in.Details != nil {
		task.SetDetails(*in.Details)
	}
	if in.Status != nil {
		if err := task.SetStatus(*in.Status); err != nil {
			var invErr *domain.InvalidTransitionError
			if errors.As(err, &invErr) {
				return nil, apperror.UnprocessableEntity(err.Error())
			}
			return nil, apperror.BadRequest(err.Error())
		}
	}
	if in.Priority != nil {
		if err := task.SetPriority(*in.Priority); err != nil {
			return nil, apperror.BadRequest(err.Error())
		}
	}
	if in.StartAt != nil || in.EndAt != nil {
		start := task.StartAt
		if in.StartAt != nil {
			start = in.StartAt
		}
		end := task.EndAt
		if in.EndAt != nil {
			end = in.EndAt
		}
		if start != nil {
			if err := task.Schedule(*start, end); err != nil {
				return nil, apperror.BadRequest(err.Error())
			}
		}
	}
	if in.Location != nil {
		task.SetLocation(*in.Location)
	}
	if in.Labels != nil {
		task.SetLabels(in.Labels)
	}
	if in.Links != nil {
		if err := task.SetLinks(in.Links); err != nil {
			return nil, apperror.BadRequest(err.Error())
		}
	}
	if in.Pinned != nil {
		if *in.Pinned {
			if err := task.Pin(); err != nil {
				return nil, apperror.BadRequest(err.Error())
			}
		} else {
			if err := task.Unpin(); err != nil {
				return nil, apperror.BadRequest(err.Error())
			}
		}
	}
	if in.Archived != nil {
		if *in.Archived {
			task.Archive()
		} else {
			task.Unarchive()
		}
	}
	if in.ParentID != nil {
		if err := task.SetParent(*in.ParentID); err != nil {
			return nil, apperror.BadRequest(err.Error())
		}
	}

	if err := s.repo.Update(ctx, task); err != nil {
		if domain.IsNotFound(err) {
			return nil, apperror.NotFound("task")
		}
		return nil, apperror.Internal(err)
	}

	s.dispatch(ctx, task)
	return task, nil
}

// Complete marks a task as completed and dispatches TaskCompletedEvent.
func (s *TaskService) Complete(ctx context.Context, id, userID uuid.UUID) (*domain.Task, error) {
	task, err := s.GetByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}

	if err := task.Complete(); err != nil {
		var invErr *domain.InvalidTransitionError
		if errors.As(err, &invErr) {
			return nil, apperror.UnprocessableEntity(err.Error())
		}
		return nil, apperror.BadRequest(err.Error())
	}

	if err := s.repo.Update(ctx, task); err != nil {
		return nil, apperror.Internal(err)
	}

	s.dispatch(ctx, task)
	return task, nil
}

// Delete permanently removes a task and cleans up its R2 attachments.
func (s *TaskService) Delete(ctx context.Context, id, userID uuid.UUID) error {
	task, err := s.GetByID(ctx, id, userID)
	if err != nil {
		return err
	}

	for _, att := range task.Attachments {
		// Log but don't fail — orphaned objects can be cleaned by R2 lifecycle rules.
		_ = s.storage.Delete(ctx, att.StorageKey)
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		if domain.IsNotFound(err) {
			return apperror.NotFound("task")
		}
		return apperror.Internal(err)
	}
	return nil
}

// Archive soft-deletes a task without permanently removing it.
func (s *TaskService) Archive(ctx context.Context, id, userID uuid.UUID) (*domain.Task, error) {
	task, err := s.GetByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	task.Archive()
	if err := s.repo.Update(ctx, task); err != nil {
		return nil, apperror.Internal(err)
	}
	s.dispatch(ctx, task)
	return task, nil
}

// ListChildren returns all direct sub-tasks of the given parent.
func (s *TaskService) ListChildren(ctx context.Context, parentID, userID uuid.UUID) ([]*domain.Task, error) {
	if _, err := s.GetByID(ctx, parentID, userID); err != nil {
		return nil, err
	}
	tasks, err := s.repo.ListByParent(ctx, parentID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return tasks, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (s *TaskService) dispatch(ctx context.Context, task *domain.Task) {
	for _, evt := range task.DomainEvents() {
		s.dispatcher.Dispatch(ctx, evt)
	}
}
