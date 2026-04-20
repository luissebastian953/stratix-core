package handler

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/luissebastian953/stratix-core/internal/domain"
	"github.com/luissebastian953/stratix-core/internal/tasks/service"
	"github.com/luissebastian953/stratix-core/pkg/pagination"
)

// ── Requests ─────────────────────────────────────────────────────────────────

type CreateTaskRequest struct {
	Type           string                  `json:"type"`
	Title          string                  `json:"title"`
	Details        string                  `json:"details"`
	Priority       string                  `json:"priority"`
	OccurrenceType string                  `json:"occurrence_type"`
	StartAt        *time.Time              `json:"start_at"`
	EndAt          *time.Time              `json:"end_at"`
	Location       string                  `json:"location"`
	Labels         []string                `json:"labels"`
	Links          []TaskLinkRequest       `json:"links"`
	ParentID       string                  `json:"parent_id"`
	RecurrenceRule *RecurrenceRuleRequest  `json:"recurrence_rule"`
}

func (r *CreateTaskRequest) Validate() error {
	r.Title = strings.TrimSpace(r.Title)
	r.Type = strings.TrimSpace(r.Type)

	if r.Title == "" {
		return errors.New("title is required")
	}
	if r.Type == "" {
		return errors.New("type is required")
	}
	if r.OccurrenceType == "" {
		r.OccurrenceType = string(domain.OccurrenceOnce)
	}
	return nil
}

func (r *CreateTaskRequest) toServiceInput() service.CreateInput {
	in := service.CreateInput{
		Type:           domain.TaskType(r.Type),
		Title:          r.Title,
		Details:        r.Details,
		OccurrenceType: domain.OccurrenceType(r.OccurrenceType),
		StartAt:        r.StartAt,
		EndAt:          r.EndAt,
		Location:       r.Location,
		Labels:         r.Labels,
	}

	if r.Priority != "" {
		p := domain.Priority(r.Priority)
		in.Priority = &p
	}

	for _, l := range r.Links {
		in.Links = append(in.Links, domain.TaskLink{Label: l.Label, URL: l.URL})
	}

	if r.ParentID != "" {
		if id, err := uuid.Parse(r.ParentID); err == nil {
			in.ParentID = &id
		}
	}

	if r.RecurrenceRule != nil {
		rule := r.RecurrenceRule.toDomain()
		in.RecurrenceRule = &rule
	}

	return in
}

type UpdateTaskRequest struct {
	Title    *string           `json:"title"`
	Details  *string           `json:"details"`
	Status   *string           `json:"status"`
	Priority *string           `json:"priority"`
	StartAt  *time.Time        `json:"start_at"`
	EndAt    *time.Time        `json:"end_at"`
	Location *string           `json:"location"`
	Labels   []string          `json:"labels"`
	Links    []TaskLinkRequest `json:"links"`
	Pinned   *bool             `json:"pinned"`
	Archived *bool             `json:"archived"`
	ParentID *string           `json:"parent_id"`
}

func (r *UpdateTaskRequest) Validate() error {
	if r.Title != nil {
		trimmed := strings.TrimSpace(*r.Title)
		if trimmed == "" {
			return errors.New("title cannot be empty")
		}
		r.Title = &trimmed
	}
	return nil
}

func (r *UpdateTaskRequest) toServiceInput() service.UpdateInput {
	in := service.UpdateInput{
		Title:    r.Title,
		Details:  r.Details,
		StartAt:  r.StartAt,
		EndAt:    r.EndAt,
		Location: r.Location,
		Pinned:   r.Pinned,
		Archived: r.Archived,
	}

	if r.Status != nil {
		s := domain.Status(*r.Status)
		in.Status = &s
	}
	if r.Priority != nil {
		p := domain.Priority(*r.Priority)
		in.Priority = &p
	}
	if r.Labels != nil {
		in.Labels = r.Labels
	}
	if r.Links != nil {
		links := make([]domain.TaskLink, len(r.Links))
		for i, l := range r.Links {
			links[i] = domain.TaskLink{Label: l.Label, URL: l.URL}
		}
		in.Links = links
	}
	if r.ParentID != nil {
		if id, err := uuid.Parse(*r.ParentID); err == nil {
			in.ParentID = &id
		}
	}

	return in
}

type TaskLinkRequest struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

type RecurrenceRuleRequest struct {
	Frequency  string     `json:"frequency"`
	Interval   int        `json:"interval"`
	ByWeekday  []int      `json:"by_weekday,omitempty"`
	ByMonthDay []int      `json:"by_month_day,omitempty"`
	ByMonth    []int      `json:"by_month,omitempty"`
	BySetPos   []int      `json:"by_set_pos,omitempty"`
	Until      *time.Time `json:"until,omitempty"`
}

func (r *RecurrenceRuleRequest) toDomain() domain.RecurrenceRule {
	weekdays := make([]time.Weekday, len(r.ByWeekday))
	for i, d := range r.ByWeekday {
		weekdays[i] = time.Weekday(d)
	}

	months := make([]time.Month, len(r.ByMonth))
	for i, m := range r.ByMonth {
		months[i] = time.Month(m)
	}

	return domain.RecurrenceRule{
		Frequency:  domain.RecurrenceType(r.Frequency),
		Interval:   r.Interval,
		ByWeekday:  weekdays,
		ByMonthDay: r.ByMonthDay,
		ByMonth:    months,
		BySetPos:   r.BySetPos,
		Until:      r.Until,
	}
}

// ── Responses ─────────────────────────────────────────────────────────────────

type TaskResponse struct {
	ID              string                  `json:"id"`
	UserID          string                  `json:"user_id"`
	ParentID        *string                 `json:"parent_id,omitempty"`
	Type            string                  `json:"type"`
	Title           string                  `json:"title"`
	Details         string                  `json:"details"`
	Status          *string                 `json:"status"`
	EffectiveStatus *string                 `json:"effective_status"`
	Priority        *string                 `json:"priority"`
	Pinned          bool                    `json:"pinned"`
	Archived        bool                    `json:"archived"`
	OccurrenceType  string                  `json:"occurrence_type"`
	StartAt         *time.Time              `json:"start_at"`
	EndAt           *time.Time              `json:"end_at"`
	RecurrenceRule  *RecurrenceRuleResponse `json:"recurrence_rule,omitempty"`
	Location        string                  `json:"location"`
	Labels          []string                `json:"labels"`
	Links           []TaskLinkResponse      `json:"links"`
	Attachments     []AttachmentResponse    `json:"attachments"`
	CreatedAt       time.Time               `json:"created_at"`
	UpdatedAt       time.Time               `json:"updated_at"`
}

type RecurrenceRuleResponse struct {
	Frequency  string     `json:"frequency"`
	Interval   int        `json:"interval"`
	ByWeekday  []int      `json:"by_weekday,omitempty"`
	ByMonthDay []int      `json:"by_month_day,omitempty"`
	ByMonth    []int      `json:"by_month,omitempty"`
	BySetPos   []int      `json:"by_set_pos,omitempty"`
	Until      *time.Time `json:"until,omitempty"`
}

type TaskLinkResponse struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

type AttachmentResponse struct {
	ID         string    `json:"id"`
	StorageKey string    `json:"storage_key"`
	Filename   string    `json:"filename"`
	MimeType   string    `json:"mime_type"`
	SizeBytes  int64     `json:"size_bytes"`
	UploadedAt time.Time `json:"uploaded_at"`
}

type TaskListResponse = pagination.Page[TaskResponse]

// ── Converters ────────────────────────────────────────────────────────────────

func toTaskResponse(t *domain.Task) TaskResponse {
	resp := TaskResponse{
		ID:             t.ID.String(),
		UserID:         t.UserID.String(),
		Type:           string(t.Type),
		Title:          t.Title,
		Details:        t.Details,
		Pinned:         t.Pinned,
		Archived:       t.Archived,
		OccurrenceType: string(t.OccurrenceType),
		StartAt:        t.StartAt,
		EndAt:          t.EndAt,
		Location:       t.Location,
		Labels:         t.Labels,
		CreatedAt:      t.CreatedAt,
		UpdatedAt:      t.UpdatedAt,
	}

	if t.ParentID != nil {
		s := t.ParentID.String()
		resp.ParentID = &s
	}

	if t.Status != nil {
		s := string(*t.Status)
		resp.Status = &s
	}

	if eff := t.EffectiveStatus(); eff != nil {
		s := string(*eff)
		resp.EffectiveStatus = &s
	}

	if t.Priority != nil {
		s := string(*t.Priority)
		resp.Priority = &s
	}

	if t.RecurrenceRule != nil {
		resp.RecurrenceRule = toRecurrenceRuleResponse(t.RecurrenceRule)
	}

	resp.Links = make([]TaskLinkResponse, len(t.Links))
	for i, l := range t.Links {
		resp.Links[i] = TaskLinkResponse{Label: l.Label, URL: l.URL}
	}

	resp.Attachments = make([]AttachmentResponse, len(t.Attachments))
	for i, a := range t.Attachments {
		resp.Attachments[i] = AttachmentResponse{
			ID:         a.ID,
			StorageKey: a.StorageKey,
			Filename:   a.Filename,
			MimeType:   a.MimeType,
			SizeBytes:  a.SizeBytes,
			UploadedAt: a.UploadedAt,
		}
	}

	return resp
}

func toRecurrenceRuleResponse(r *domain.RecurrenceRule) *RecurrenceRuleResponse {
	weekdays := make([]int, len(r.ByWeekday))
	for i, d := range r.ByWeekday {
		weekdays[i] = int(d)
	}

	months := make([]int, len(r.ByMonth))
	for i, m := range r.ByMonth {
		months[i] = int(m)
	}

	return &RecurrenceRuleResponse{
		Frequency:  string(r.Frequency),
		Interval:   r.Interval,
		ByWeekday:  weekdays,
		ByMonthDay: r.ByMonthDay,
		ByMonth:    months,
		BySetPos:   r.BySetPos,
		Until:      r.Until,
	}
}

func toTaskListResponse(page pagination.Page[*domain.Task]) TaskListResponse {
	items := make([]TaskResponse, len(page.Items))
	for i, t := range page.Items {
		items[i] = toTaskResponse(t)
	}
	return pagination.Page[TaskResponse]{
		Items:      items,
		NextCursor: page.NextCursor,
		HasMore:    page.HasMore,
	}
}
