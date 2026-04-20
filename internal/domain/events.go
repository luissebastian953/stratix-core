package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type HandlerFunc func(ctx context.Context, event DomainEvent) error

type DomainEvent interface {
	EventName() string
}

// ── Event types ───────────────────────────────────────────────────────────────

type TaskCreatedEvent struct {
	TaskID   uuid.UUID
	UserID   uuid.UUID
	TaskType TaskType
	StartAt  *time.Time
}

func (e TaskCreatedEvent) EventName() string { return "task.created" }

type TaskCompletedEvent struct {
	TaskID      uuid.UUID
	UserID      uuid.UUID
	CompletedAt time.Time
}

func (e TaskCompletedEvent) EventName() string { return "task.completed" }

type TaskArchivedEvent struct {
	TaskID uuid.UUID
	UserID uuid.UUID
}

func (e TaskArchivedEvent) EventName() string { return "task.archived" }

type TaskRescheduledEvent struct {
	TaskID         uuid.UUID
	UserID         uuid.UUID
	NewStartAt     *time.Time
	OccurrenceType OccurrenceType
}

func (e TaskRescheduledEvent) EventName() string { return "task.rescheduled" }

type TaskOverdueEvent struct {
	TaskID uuid.UUID
	UserID uuid.UUID
	EndAt  time.Time
}

func (e TaskOverdueEvent) EventName() string { return "task.overdue" }

type RecurrenceInstanceCompletedEvent struct {
	TaskID      uuid.UUID
	UserID      uuid.UUID
	InstanceID  uuid.UUID
	ScheduledAt time.Time
	CompletedAt time.Time
}

func (e RecurrenceInstanceCompletedEvent) EventName() string {
	return "recurrence_instance.completed"
}
