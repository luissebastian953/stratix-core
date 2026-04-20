package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/luissebastian953/stratix-core/internal/domain"
	"github.com/luissebastian953/stratix-core/internal/events"
	"github.com/luissebastian953/stratix-core/pkg/apperror"
)

// RecurrenceService manages recurrence instance lifecycle.
// Instances are generated lazily on calendar queries — only stored when
// the user acts on them (complete/skip) or when the scheduler detects overdue.
type RecurrenceService struct {
	tasks      domain.TaskRepository
	recur      domain.RecurrenceRepository
	dispatcher *events.Dispatcher
}

func NewRecurrenceService(
	tasks domain.TaskRepository,
	recur domain.RecurrenceRepository,
	dispatcher *events.Dispatcher,
) *RecurrenceService {
	return &RecurrenceService{tasks: tasks, recur: recur, dispatcher: dispatcher}
}

// GetInstancesInRange returns all recurrence instances for a task in [from, to).
// Pending instances that don't yet exist in the DB are generated and persisted.
func (s *RecurrenceService) GetInstancesInRange(
	ctx context.Context,
	taskID, userID uuid.UUID,
	from, to time.Time,
) ([]*domain.RecurrenceInstance, error) {
	task, err := s.tasks.GetByID(ctx, taskID)
	if err != nil {
		if domain.IsNotFound(err) {
			return nil, apperror.NotFound("task")
		}
		return nil, apperror.Internal(err)
	}
	if task.UserID != userID {
		return nil, apperror.NotFound("task")
	}
	if !task.IsRecurring() {
		return nil, apperror.BadRequest("task is not recurring")
	}

	stored, err := s.recur.ListInstancesInRange(ctx, taskID, from, to)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	// Index existing instances by their scheduled date (day precision in UTC)
	// so we can skip generating duplicates.
	existing := make(map[string]struct{}, len(stored))
	for _, inst := range stored {
		existing[dayKey(inst.ScheduledAt)] = struct{}{}
	}

	occurrences := generateOccurrences(*task.RecurrenceRule, *task.StartAt, from, to)
	for _, occ := range occurrences {
		if _, ok := existing[dayKey(occ)]; ok {
			continue
		}
		inst := &domain.RecurrenceInstance{
			ID:          uuid.New(),
			TaskID:      taskID,
			ScheduledAt: occ,
			Status:      domain.InstancePending,
		}
		if err := s.recur.SaveInstance(ctx, inst); err != nil {
			return nil, apperror.Internal(err)
		}
		stored = append(stored, inst)
	}

	// Sort by scheduled date ascending
	sortInstances(stored)
	return stored, nil
}

// CompleteInstance marks a recurrence instance as completed.
func (s *RecurrenceService) CompleteInstance(
	ctx context.Context,
	instanceID, taskID, userID uuid.UUID,
) (*domain.RecurrenceInstance, error) {
	inst, err := s.getOwnedInstance(ctx, instanceID, taskID, userID)
	if err != nil {
		return nil, err
	}

	if inst.IsCompleted() {
		return inst, nil
	}

	inst.Complete()

	if err := s.recur.UpdateInstance(ctx, inst); err != nil {
		return nil, apperror.Internal(err)
	}

	s.dispatcher.Dispatch(ctx, domain.RecurrenceInstanceCompletedEvent{
		TaskID:      taskID,
		UserID:      userID,
		InstanceID:  inst.ID,
		ScheduledAt: inst.ScheduledAt,
		CompletedAt: *inst.CompletedAt,
	})

	return inst, nil
}

// SkipInstance marks a recurrence instance as skipped.
func (s *RecurrenceService) SkipInstance(
	ctx context.Context,
	instanceID, taskID, userID uuid.UUID,
) (*domain.RecurrenceInstance, error) {
	inst, err := s.getOwnedInstance(ctx, instanceID, taskID, userID)
	if err != nil {
		return nil, err
	}

	if inst.IsSkipped() {
		return inst, nil
	}

	inst.Skip()

	if err := s.recur.UpdateInstance(ctx, inst); err != nil {
		return nil, apperror.Internal(err)
	}

	return inst, nil
}

// ProcessOverdue is called by the background scheduler.
// It emits TaskOverdueEvents for pending instances whose scheduled_at has passed.
func (s *RecurrenceService) ProcessOverdue(ctx context.Context, limit int) error {
	instances, err := s.recur.ListPendingOverdue(ctx, limit)
	if err != nil {
		return err
	}

	for _, inst := range instances {
		task, err := s.tasks.GetByID(ctx, inst.TaskID)
		if err != nil {
			continue
		}
		s.dispatcher.Dispatch(ctx, domain.TaskOverdueEvent{
			TaskID: inst.TaskID,
			UserID: task.UserID,
			EndAt:  inst.ScheduledAt,
		})
	}

	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (s *RecurrenceService) getOwnedInstance(
	ctx context.Context,
	instanceID, taskID, userID uuid.UUID,
) (*domain.RecurrenceInstance, error) {
	task, err := s.tasks.GetByID(ctx, taskID)
	if err != nil {
		if domain.IsNotFound(err) {
			return nil, apperror.NotFound("task")
		}
		return nil, apperror.Internal(err)
	}
	if task.UserID != userID {
		return nil, apperror.NotFound("task")
	}

	inst, err := s.recur.GetInstance(ctx, instanceID)
	if err != nil {
		if domain.IsNotFound(err) {
			return nil, apperror.NotFound("recurrence instance")
		}
		return nil, apperror.Internal(err)
	}
	if inst.TaskID != taskID {
		return nil, apperror.NotFound("recurrence instance")
	}

	return inst, nil
}

// generateOccurrences computes occurrence datetimes in [from, to) for a rule.
// The rule's StartAt anchors the series — only dates >= from are returned.
func generateOccurrences(rule domain.RecurrenceRule, start, from, to time.Time) []time.Time {
	if rule.Until != nil && rule.Until.Before(from) {
		return nil
	}

	interval := rule.Interval
	if interval <= 0 {
		interval = 1
	}

	var results []time.Time
	cur := start.UTC()

	// Advance cur to the first occurrence on or after from.
	for cur.Before(from) {
		cur = nextOccurrence(rule, cur, interval)
	}

	ceiling := to
	if rule.Until != nil && rule.Until.Before(ceiling) {
		ceiling = *rule.Until
	}

	for !cur.After(ceiling) && !cur.After(to) {
		if !cur.Before(from) {
			results = append(results, cur)
		}
		cur = nextOccurrence(rule, cur, interval)
	}

	return results
}

func nextOccurrence(rule domain.RecurrenceRule, cur time.Time, interval int) time.Time {
	switch rule.Frequency {
	case domain.RecurrenceTypeDaily:
		return cur.AddDate(0, 0, interval)
	case domain.RecurrenceTypeWeekly:
		if len(rule.ByWeekday) == 0 {
			return cur.AddDate(0, 0, 7*interval)
		}
		// Advance day by day until we hit a matching weekday within the interval window.
		next := cur.AddDate(0, 0, 1)
		weekStart := cur.AddDate(0, 0, -int(cur.Weekday()))
		weekEnd := weekStart.AddDate(0, 0, 7*interval)
		for next.Before(weekEnd) {
			for _, wd := range rule.ByWeekday {
				if next.Weekday() == wd && next.After(cur) {
					return next
				}
			}
			next = next.AddDate(0, 0, 1)
		}
		return weekEnd
	case domain.RecurrenceTypeMonthly:
		return cur.AddDate(0, interval, 0)
	default:
		return cur.AddDate(0, 0, interval)
	}
}

func dayKey(t time.Time) string {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
}

func sortInstances(instances []*domain.RecurrenceInstance) {
	for i := 1; i < len(instances); i++ {
		for j := i; j > 0 && instances[j].ScheduledAt.Before(instances[j-1].ScheduledAt); j-- {
			instances[j], instances[j-1] = instances[j-1], instances[j]
		}
	}
}
