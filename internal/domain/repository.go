package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type TaskFilter struct {
	Types    []TaskType
	Statuses []Status
	Labels   []string

	From     *time.Time
	To       *time.Time
	ParentID *uuid.UUID

	OnlyRoots bool
	Archived  *bool
	Pinned    *bool

	Search string
	Cursor string
	Limit  int
}

type RecurrenceInstance struct {
	ID          uuid.UUID
	TaskID      uuid.UUID
	ScheduledAt time.Time
	Status      InstanceStatus
	CompletedAt *time.Time
}

func (r *RecurrenceInstance) IsCompleted() bool {
	return r.Status == InstanceCompleted
}

func (r *RecurrenceInstance) IsSkipped() bool {
	return r.Status == InstanceSkipped
}

func (r *RecurrenceInstance) Complete() {
	r.Status = InstanceCompleted
	now := time.Now().UTC()
	r.CompletedAt = &now
}

func (r *RecurrenceInstance) Skip() {
	r.Status = InstanceSkipped
}

type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

type Device struct {
	DeviceID  string
	UserID    uuid.UUID
	Token     string
	Platform  string
	UpdatedAt time.Time
}

func (d *Device) IsIOS() bool {
	return d.Platform == "ios"
}

func (d *Device) IsAndroid() bool {
	return d.Platform == "android"
}

type TaskRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*Task, error)

	ListByUser(ctx context.Context, userID uuid.UUID, filter TaskFilter) ([]*Task, error)

	Save(ctx context.Context, task *Task) error

	Update(ctx context.Context, task *Task) error

	Delete(ctx context.Context, id uuid.UUID) error

	ListByParent(ctx context.Context, parentID uuid.UUID) ([]*Task, error)

	CountByUser(ctx context.Context, userID uuid.UUID, filter TaskFilter) (int, error)
}

type RecurrenceRepository interface {
	ListInstancesInRange(ctx context.Context, taskID uuid.UUID, from, to time.Time) ([]*RecurrenceInstance, error)

	GetInstance(ctx context.Context, id uuid.UUID) (*RecurrenceInstance, error)

	SaveInstance(ctx context.Context, instance *RecurrenceInstance) error

	UpdateInstance(ctx context.Context, instance *RecurrenceInstance) error

	DeleteByTaskID(ctx context.Context, taskID uuid.UUID) error

	ListPendingOverdue(ctx context.Context, limit int) ([]*RecurrenceInstance, error)
}

type UserRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)

	GetByEmail(ctx context.Context, email string) (*User, error)

	Create(ctx context.Context, user *User) error

	UpdatePasswordHash(ctx context.Context, userID uuid.UUID, hash string) error
}

type DeviceRepository interface {
	Upsert(ctx context.Context, device *Device) error

	GetByUser(ctx context.Context, userID uuid.UUID) ([]*Device, error)

	Delete(ctx context.Context, deviceID string) error

	DeleteByUser(ctx context.Context, userID uuid.UUID) error
}

type AnalyticsEvent struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	EventType  string
	Payload    map[string]any
	OccurredAt time.Time
}

type TaskSummary struct {
	Total     int `json:"total"`
	Completed int `json:"completed"`
	Overdue   int `json:"overdue"`
	Archived  int `json:"archived"`
}

type TypeCount struct {
	Type  string `json:"type"`
	Count int    `json:"count"`
}

type AnalyticsRepository interface {
	Save(ctx context.Context, event *AnalyticsEvent) error

	Summary(ctx context.Context, userID uuid.UUID) (*TaskSummary, error)

	TypeDistribution(ctx context.Context, userID uuid.UUID) ([]TypeCount, error)

	// CompletedPerDay returns the count of completed tasks per day for the last n days.
	CompletedPerDay(ctx context.Context, userID uuid.UUID, days int) (map[string]int, error)
}

type Notification struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TaskID    *uuid.UUID
	Channel   string
	Title     string
	Body      string
	SentAt    time.Time
	CreatedAt time.Time
}

type NotificationRepository interface {
	Save(ctx context.Context, n *Notification) error

	ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]*Notification, error)
}

type RefreshToken struct {
	Token     string
	UserID    uuid.UUID
	ExpiresAt time.Time
	Revoked   bool
	CreatedAt time.Time
}

type RefreshTokenRepository interface {
	Store(ctx context.Context, token string, userID uuid.UUID, expiresAt time.Time) error

	Get(ctx context.Context, token string) (*RefreshToken, error)

	Revoke(ctx context.Context, token string) error

	RevokeAllForUser(ctx context.Context, userID uuid.UUID) error

	DeleteExpired(ctx context.Context) error
}

type NotFoundError struct {
	Resource string
	ID       string
}

func (e *NotFoundError) Error() string {
	return e.Resource + " not found: " + e.ID
}

type ConflictError struct {
	Resource string
	Field    string
}

func (e *ConflictError) Error() string {
	return e.Resource + " already exists: " + e.Field
}

func IsNotFound(err error) bool {
	if err == nil {
		return false
	}

	_, ok := err.(*NotFoundError)

	return ok
}

func IsConflict(err error) bool {
	if err == nil {
		return false
	}

	_, ok := err.(*ConflictError)

	return ok
}
