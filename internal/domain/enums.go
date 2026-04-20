package domain

// define enums
type TaskType string

const (
	TaskTypeTodo     TaskType = "todo"
	TaskTypeTravel   TaskType = "travel"
	TaskTypeMeeting  TaskType = "meeting"
	TaskTypeReminder TaskType = "reminder"
	TaskTypeFocus    TaskType = "focus"
	TaskTypeLog      TaskType = "log"
)

type Priority string

const (
	PriorityHighest Priority = "highest"
	PriorityHigh    Priority = "high"
	PriorityMedium  Priority = "medium"
	PriorityLow     Priority = "low"
	PriorityLowest  Priority = "lowest"
)

type OccurrenceType string

const (
	OccurrenceOnce      OccurrenceType = "once"
	OccurrenceRecurring OccurrenceType = "recurring"
	OccurrenceUnbound   OccurrenceType = "unbound"
)

type Status string

const (
	StatusBacklog    Status = "backlog"
	StatusScheduled  Status = "scheduled"
	StatusInProgress Status = "in_progress"
	StatusCompleted  Status = "completed"
	StatusDuePassed  Status = "due_passed"
	StatusCancelled  Status = "cancelled"
)

type InstanceStatus string

const (
	InstancePending   InstanceStatus = "pending"
	InstanceCompleted InstanceStatus = "completed"
	InstanceSkipped   InstanceStatus = "skipped"
)
