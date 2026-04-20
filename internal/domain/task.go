package domain

import (
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Task is the core aggregate of the stratix domain.
type Task struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	ParentID       *uuid.UUID
	Type           TaskType
	Title          string
	Details        string
	Status         *Status   // nil for Log type
	Priority       *Priority // nil for Log type
	Pinned         bool      // meaningful only for Log type
	Archived       bool
	OccurrenceType OccurrenceType
	StartAt        *time.Time
	EndAt          *time.Time
	RecurrenceRule *RecurrenceRule // non-nil only when OccurrenceType == OccurrenceRecurring
	Location       string
	Labels         []string
	Links          []TaskLink
	Attachments    []TaskAttachmentRef
	CreatedAt      time.Time
	UpdatedAt      time.Time

	events []DomainEvent
}

// behaviour
func (t *Task) EventName() string {
	return string(t.Type)
}

func (t *Task) NewTask(userID uuid.UUID, taskType TaskType, title string) (*Task, error) {
	if err := validateTitle(title); err != nil {
		return nil, err
	}

	if err := validateType(taskType); err != nil {
		return nil, err
	}

	now := time.Now()

	t = &Task{
		ID:        uuid.New(),
		UserID:    userID,
		Type:      taskType,
		Title:     strings.TrimSpace(title),
		Labels:    []string{},
		Links:     []TaskLink{},
		CreatedAt: now,
		UpdatedAt: now,
	}

	t.applyTypeDefaults()

	t.addEvent(TaskCreatedEvent{
		TaskID:   t.ID,
		UserID:   t.UserID,
		TaskType: t.Type,
		StartAt:  t.StartAt,
	})

	return t, nil
}

func (t *Task) SetTitle(title string) error {
	if err := validateTitle(title); err != nil {
		return err
	}

	t.Title = strings.TrimSpace(title)
	t.touch()

	return nil
}

func (t *Task) SetDetails(details string) error {
	t.Details = strings.TrimSpace(details)
	t.touch()

	return nil
}

func (t *Task) SetStatus(next Status) error {
	if t.Type == TaskTypeLog {
		return errors.New("log tasks do not have a status")
	}

	if t.Status == nil {
		return errors.New("task has no current status")
	}

	// idempotent — not an error
	if *t.Status == next {
		return nil
	}

	if !isValidTransition(*t.Status, next) {
		return &InvalidTransitionError{From: *t.Status, To: next}
	}

	t.Status = &next
	t.touch()

	return nil
}

func (t *Task) SetPriority(priority Priority) error {
	if t.Type == TaskTypeLog {
		return errors.New("log tasks do not have a priority")
	}

	if validatePriority(priority) != nil {
		return errors.New("invalid priority")
	}

	t.Priority = &priority
	t.touch()

	return nil
}

func (t *Task) SetLabels(labels []string) {
	seen := make(map[string]struct{})
	clean := make([]string, 0, len(labels))

	for _, l := range labels {
		l = strings.TrimSpace(strings.ToLower(l))
		if l == "" {
			continue
		}
		if _, dup := seen[l]; dup {
			continue
		}
		seen[l] = struct{}{}
		clean = append(clean, l)
	}

	t.Labels = clean
	t.touch()
}

func (t *Task) AddLabel(label string) {
	label = strings.TrimSpace(strings.ToLower(label))

	if label == "" {
		return
	}

	if slices.Contains(t.Labels, label) {
		return
	}

	t.Labels = append(t.Labels, label)
	t.touch()
}

func (t *Task) RemoveLabel(label string) {
	label = strings.TrimSpace(strings.ToLower(label))
	result := t.Labels[:0]

	for _, l := range t.Labels {
		if l != label {
			result = append(result, l)
		}
	}

	t.Labels = result
	t.touch()
}

func (t *Task) SetLinks(links []TaskLink) error {
	for _, l := range links {
		if err := validateLink(l); err != nil {
			return err
		}
	}

	t.Links = links
	t.touch()

	return nil
}

func (t *Task) AddLink(link TaskLink) error {
	if err := validateLink(link); err != nil {
		return err
	}

	t.Links = append(t.Links, link)
	t.touch()

	return nil
}

func (t *Task) RemoveLink(url string) {
	result := t.Links[:0]
	for _, l := range t.Links {
		if l.URL != url {
			result = append(result, l)
		}
	}

	t.Links = result
	t.touch()
}

func (t *Task) AddAttachment(ref TaskAttachmentRef) error {
	if ref.StorageKey == "" {
		return errors.New("attachment storage key is required")
	}

	if ref.Filename == "" {
		return errors.New("attachment filename is required")
	}

	if ref.SizeBytes <= 0 {
		return errors.New("attachment size must be positive")
	}

	t.Attachments = append(t.Attachments, ref)
	t.touch()

	return nil
}

func (t *Task) RemoveAttachment(storageKey string) (TaskAttachmentRef, error) {
	for i, a := range t.Attachments {
		if a.StorageKey == storageKey {
			t.Attachments = append(t.Attachments[:i], t.Attachments[i+1:]...)
			t.touch()
			return a, nil
		}
	}

	return TaskAttachmentRef{}, errors.New("attachment not found: " + storageKey)
}

func (t *Task) SetParent(parentID uuid.UUID) error {
	if parentID == t.ID {
		return errors.New("a task cannot be its own parent")
	}

	t.ParentID = &parentID
	t.touch()

	return nil
}

func (t *Task) ClearParent() {
	t.ParentID = nil
	t.touch()
}

func (t *Task) IsOverdue() bool {
	if t.Type == TaskTypeLog {
		return false
	}

	if t.Archived {
		return false
	}

	if t.Status != nil && *t.Status == StatusCompleted {
		return false
	}

	if t.EndAt == nil {
		return false
	}

	return t.EndAt.Before(time.Now().UTC())
}

func (t *Task) IsScheduledForToday(loc *time.Location) bool {
	if t.StartAt == nil {
		return false
	}

	now := time.Now().In(loc)
	start := t.StartAt.In(loc)

	return start.Year() == now.Year() && start.Month() == now.Month() && start.Day() == now.Day()
}

func (t *Task) IsRecurring() bool {
	return t.OccurrenceType == OccurrenceRecurring && t.RecurrenceRule != nil
}

func (t *Task) EffectiveStatus() *Status {
	if t.IsOverdue() {
		s := StatusDuePassed

		return &s
	}

	return t.Status
}

func (t *Task) DomainEvents() []DomainEvent {
	evts := make([]DomainEvent, len(t.events))

	copy(evts, t.events)
	t.events = nil

	return evts
}

func (t *Task) Duration() time.Duration {
	if t.StartAt == nil || t.EndAt == nil {
		return 0
	}

	return t.EndAt.Sub(*t.StartAt)
}

func (t *Task) HasParent() bool {
	return t.ParentID != nil
}

func (t *Task) Schedule(start time.Time, end *time.Time) error {
	if t.Type == TaskTypeLog {
		return errors.New("log tasks cannot be scheduled")
	}

	if end != nil && !end.After(start) {
		return errors.New("end time must be after start time")
	}

	rescheduling := t.StartAt != nil
	t.OccurrenceType = OccurrenceOnce
	t.StartAt = timePtr(start.UTC())
	t.EndAt = utcPtrOrNil(end)
	t.RecurrenceRule = nil

	if t.Status != nil && *t.Status == StatusBacklog && start.After(time.Now().UTC()) {
		s := StatusScheduled
		t.Status = &s
	}

	if rescheduling {
		t.addEvent(TaskRescheduledEvent{
			TaskID:         t.ID,
			UserID:         t.UserID,
			NewStartAt:     t.StartAt,
			OccurrenceType: t.OccurrenceType,
		})
	}

	t.touch()

	return nil
}

func (t *Task) SetRecurring(rule RecurrenceRule, firstOccurrence time.Time) error {
	if t.Type == TaskTypeLog {
		return errors.New("log tasks cannot recur")
	}

	t.OccurrenceType = OccurrenceRecurring
	t.RecurrenceRule = &rule
	t.StartAt = timePtr(firstOccurrence.UTC())
	t.EndAt = nil
	t.touch()

	return nil
}

func (t *Task) ClearSchedule() error {
	if t.Type != TaskTypeLog {
		return errors.New("only log tasks can be unbound")
	}

	t.OccurrenceType = OccurrenceUnbound
	t.StartAt = nil
	t.EndAt = nil
	t.RecurrenceRule = nil
	t.touch()

	return nil
}

func (t *Task) SetLocation(location string) {
	t.Location = strings.TrimSpace(location)
	t.touch()
}

func (t *Task) Archive() {
	t.Archived = true
	t.addEvent(TaskArchivedEvent{TaskID: t.ID, UserID: t.UserID})
	t.touch()
}

func (t *Task) Unarchive() {
	t.Archived = false
	t.touch()
}

func (t *Task) Pin() error {
	if t.Type != TaskTypeLog {
		return errors.New("only log tasks can be pinned")
	}

	t.Pinned = true
	t.touch()

	return nil
}

func (t *Task) Unpin() error {
	if t.Type != TaskTypeLog {
		return errors.New("only log tasks can be unpinned")
	}

	t.Pinned = false
	t.touch()

	return nil
}

func (t *Task) Reopen() error {
	return t.SetStatus(StatusBacklog)
}

func (t *Task) Complete() error {
	if err := t.SetStatus(StatusCompleted); err != nil {
		return err
	}

	t.addEvent(TaskCompletedEvent{
		TaskID:      t.ID,
		UserID:      t.UserID,
		CompletedAt: t.UpdatedAt,
	})

	return nil
}

func (t *Task) addEvent(e DomainEvent) {
	t.events = append(t.events, e)
}

// validations
func validateTitle(title string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return errors.New("title is required")
	}

	if len(title) > 255 {
		return errors.New("title must be 255 characters or fewer")
	}

	return nil
}

func validateType(t TaskType) error {
	switch t {
	case TaskTypeTodo, TaskTypeTravel, TaskTypeMeeting,
		TaskTypeReminder, TaskTypeFocus, TaskTypeLog:
		return nil
	}

	return errors.New("invalid task type: " + string(t))
}

func validatePriority(p Priority) error {
	switch p {
	case PriorityHighest, PriorityHigh, PriorityMedium, PriorityLow, PriorityLowest:
		return nil
	}

	return errors.New("invalid priority: " + string(p))
}

func validateLink(l TaskLink) error {
	if strings.TrimSpace(l.URL) == "" {
		return errors.New("link URL is required")
	}

	if !strings.HasPrefix(l.URL, "http://") && !strings.HasPrefix(l.URL, "https://") {
		return errors.New("link URL must start with http:// or https://")
	}

	return nil
}

// apply default
func (t *Task) applyTypeDefaults() {
	if t.Type == TaskTypeLog {
		t.OccurrenceType = OccurrenceUnbound
		t.Status = nil
		t.Priority = nil
	} else {
		t.OccurrenceType = OccurrenceOnce
		s := StatusBacklog
		t.Status = &s
		p := PriorityMedium
		t.Priority = &p
	}
}

// transition
var validTransitions = map[Status][]Status{
	StatusBacklog:    {StatusInProgress, StatusScheduled},
	StatusScheduled:  {StatusInProgress, StatusBacklog},
	StatusInProgress: {StatusCompleted, StatusBacklog},
	StatusDuePassed:  {StatusCompleted, StatusBacklog},

	// terminal
	StatusCompleted: {},
}

func isValidTransition(from, to Status) bool {
	return slices.Contains(validTransitions[from], to)
}

// errors
type InvalidTransitionError struct {
	From Status
	To   Status
}

func (e *InvalidTransitionError) Error() string {
	return "cannot transition task from '" + string(e.From) + "' to '" + string(e.To) + "'"
}

// trigger
func (t *Task) touch() {
	t.UpdatedAt = time.Now().UTC()
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func utcPtrOrNil(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}

	utc := t.UTC()

	return &utc
}
