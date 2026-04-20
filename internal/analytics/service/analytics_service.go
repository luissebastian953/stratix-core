package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/luissebastian953/stratix-core/internal/domain"
	"github.com/luissebastian953/stratix-core/pkg/apperror"
)

// AnalyticsService records domain events and serves reporting queries.
// It subscribes to the dispatcher in bootstrap — no other module imports it.
type AnalyticsService struct {
	repo domain.AnalyticsRepository
}

func NewAnalyticsService(repo domain.AnalyticsRepository) *AnalyticsService {
	return &AnalyticsService{repo: repo}
}

// ── Event handlers (called by Dispatcher) ────────────────────────────────────

func (s *AnalyticsService) OnTaskCreated(ctx context.Context, evt domain.DomainEvent) error {
	e := evt.(domain.TaskCreatedEvent)
	return s.record(ctx, e.UserID, "task.created", map[string]any{
		"task_id":   e.TaskID,
		"task_type": string(e.TaskType),
	})
}

func (s *AnalyticsService) OnTaskCompleted(ctx context.Context, evt domain.DomainEvent) error {
	e := evt.(domain.TaskCompletedEvent)
	return s.record(ctx, e.UserID, "task.completed", map[string]any{
		"task_id":      e.TaskID,
		"completed_at": e.CompletedAt,
	})
}

func (s *AnalyticsService) OnTaskArchived(ctx context.Context, evt domain.DomainEvent) error {
	e := evt.(domain.TaskArchivedEvent)
	return s.record(ctx, e.UserID, "task.archived", map[string]any{
		"task_id": e.TaskID,
	})
}

func (s *AnalyticsService) OnTaskRescheduled(ctx context.Context, evt domain.DomainEvent) error {
	e := evt.(domain.TaskRescheduledEvent)
	return s.record(ctx, e.UserID, "task.rescheduled", map[string]any{
		"task_id":         e.TaskID,
		"new_start_at":    e.NewStartAt,
		"occurrence_type": string(e.OccurrenceType),
	})
}

func (s *AnalyticsService) OnRecurrenceInstanceCompleted(ctx context.Context, evt domain.DomainEvent) error {
	e := evt.(domain.RecurrenceInstanceCompletedEvent)
	return s.record(ctx, e.UserID, "recurrence_instance.completed", map[string]any{
		"task_id":      e.TaskID,
		"instance_id":  e.InstanceID,
		"scheduled_at": e.ScheduledAt,
		"completed_at": e.CompletedAt,
	})
}

// ── Query use cases ───────────────────────────────────────────────────────────

func (s *AnalyticsService) Summary(ctx context.Context, userID uuid.UUID) (*domain.TaskSummary, error) {
	summary, err := s.repo.Summary(ctx, userID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return summary, nil
}

func (s *AnalyticsService) TypeDistribution(ctx context.Context, userID uuid.UUID) ([]domain.TypeCount, error) {
	counts, err := s.repo.TypeDistribution(ctx, userID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return counts, nil
}

// CompletedPerDay returns completion counts per day for the last n days.
// Used to render streaks and activity charts in the mobile app.
func (s *AnalyticsService) CompletedPerDay(ctx context.Context, userID uuid.UUID, days int) (map[string]int, error) {
	if days <= 0 || days > 365 {
		days = 30
	}
	counts, err := s.repo.CompletedPerDay(ctx, userID, days)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return counts, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (s *AnalyticsService) record(ctx context.Context, userID uuid.UUID, eventType string, payload map[string]any) error {
	return s.repo.Save(ctx, &domain.AnalyticsEvent{
		ID:         uuid.New(),
		UserID:     userID,
		EventType:  eventType,
		Payload:    payload,
		OccurredAt: time.Now().UTC(),
	})
}
