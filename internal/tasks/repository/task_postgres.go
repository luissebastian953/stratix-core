package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/luissebastian953/stratix-core/internal/domain"
)

// postgresTaskRepository implements domain.TaskRepository using pgx/v5.
//
// Query strategy:
//   - Simple lookups (GetByID, ListByParent) use QueryRow / Query directly.
//   - ListByUser builds a dynamic WHERE clause from TaskFilter fields so
//     a single SQL function handles all filter combinations cleanly.
//   - All list queries fetch limit+1 rows — if len(rows) > limit, HasMore=true.
//     The repository returns the raw slice; NewPage() in the handler trims it.
type postgresTaskRepository struct {
	db *pgxpool.Pool
}

func NewTaskRepository(db *pgxpool.Pool) domain.TaskRepository {
	return &postgresTaskRepository{db: db}
}

// ── domain.TaskRepository ─────────────────────────────────────────────────────

func (r *postgresTaskRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Task, error) {
	row := r.db.QueryRow(ctx, `
		SELECT
			id, user_id, parent_id, type, title, details,
			status, priority, pinned, archived,
			occurrence_type, start_at, end_at, recurrence_rule,
			location, labels, created_at, updated_at
		FROM tasks
		WHERE id = $1
		LIMIT 1
	`, id)

	task, err := scanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.NotFoundError{Resource: "task", ID: id.String()}
		}
		return nil, fmt.Errorf("GetByID: %w", err)
	}

	if err := r.hydrateRelations(ctx, task); err != nil {
		return nil, err
	}
	return task, nil
}

func (r *postgresTaskRepository) ListByUser(ctx context.Context, userID uuid.UUID, filter domain.TaskFilter) ([]*domain.Task, error) {
	query, args := buildListQuery(userID, filter)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("ListByUser: %w", err)
	}
	defer rows.Close()

	var tasks []*domain.Task
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("ListByUser scan: %w", err)
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ListByUser rows: %w", err)
	}

	// Hydrate links and attachments for each task.
	// This is an N+1 in the naive sense but is acceptable here because:
	//   a) list pages are capped at 100 items
	//   b) links/attachments are rarely populated on most tasks
	// If profiling shows this is slow, replace with a single JOIN query.
	for _, task := range tasks {
		if err := r.hydrateRelations(ctx, task); err != nil {
			return nil, err
		}
	}

	if tasks == nil {
		tasks = []*domain.Task{}
	}
	return tasks, nil
}

func (r *postgresTaskRepository) Save(ctx context.Context, task *domain.Task) error {
	recurrenceJSON, err := marshalRecurrenceRule(task.RecurrenceRule)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO tasks (
			id, user_id, parent_id, type, title, details,
			status, priority, pinned, archived,
			occurrence_type, start_at, end_at, recurrence_rule,
			location, labels, created_at, updated_at
		) VALUES (
			$1,  $2,  $3,  $4,  $5,  $6,
			$7,  $8,  $9,  $10,
			$11, $12, $13, $14,
			$15, $16, $17, $18
		)
	`,
		task.ID, task.UserID, task.ParentID, string(task.Type), task.Title, task.Details,
		statusPtr(task.Status), priorityPtr(task.Priority), task.Pinned, task.Archived,
		string(task.OccurrenceType), task.StartAt, task.EndAt, recurrenceJSON,
		task.Location, task.Labels, task.CreatedAt, task.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("Save task: %w", err)
	}

	if err := r.saveRelations(ctx, task); err != nil {
		return err
	}
	return nil
}

func (r *postgresTaskRepository) Update(ctx context.Context, task *domain.Task) error {
	recurrenceJSON, err := marshalRecurrenceRule(task.RecurrenceRule)
	if err != nil {
		return err
	}

	result, err := r.db.Exec(ctx, `
		UPDATE tasks SET
			parent_id       = $2,
			title           = $3,
			details         = $4,
			status          = $5,
			priority        = $6,
			pinned          = $7,
			archived        = $8,
			occurrence_type = $9,
			start_at        = $10,
			end_at          = $11,
			recurrence_rule = $12,
			location        = $13,
			labels          = $14,
			updated_at      = $15
		WHERE id = $1
	`,
		task.ID, task.ParentID, task.Title, task.Details,
		statusPtr(task.Status), priorityPtr(task.Priority), task.Pinned, task.Archived,
		string(task.OccurrenceType), task.StartAt, task.EndAt, recurrenceJSON,
		task.Location, task.Labels, task.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("Update task: %w", err)
	}
	if result.RowsAffected() == 0 {
		return &domain.NotFoundError{Resource: "task", ID: task.ID.String()}
	}

	// Replace links and attachments — delete existing then re-insert.
	// This is the simplest correct approach for small collections.
	if err := r.deleteRelations(ctx, task.ID); err != nil {
		return err
	}
	if err := r.saveRelations(ctx, task); err != nil {
		return err
	}
	return nil
}

func (r *postgresTaskRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// task_links, task_attachments, recurrence_instances are CASCADE deleted
	// by the foreign key constraint defined in 002_create_tasks.up.sql.
	result, err := r.db.Exec(ctx, `DELETE FROM tasks WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("Delete task: %w", err)
	}
	if result.RowsAffected() == 0 {
		return &domain.NotFoundError{Resource: "task", ID: id.String()}
	}
	return nil
}

func (r *postgresTaskRepository) ListByParent(ctx context.Context, parentID uuid.UUID) ([]*domain.Task, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			id, user_id, parent_id, type, title, details,
			status, priority, pinned, archived,
			occurrence_type, start_at, end_at, recurrence_rule,
			location, labels, created_at, updated_at
		FROM tasks
		WHERE parent_id = $1
		ORDER BY created_at ASC
	`, parentID)
	if err != nil {
		return nil, fmt.Errorf("ListByParent: %w", err)
	}
	defer rows.Close()

	var tasks []*domain.Task
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("ListByParent scan: %w", err)
		}
		tasks = append(tasks, task)
	}
	if tasks == nil {
		tasks = []*domain.Task{}
	}
	return tasks, nil
}

func (r *postgresTaskRepository) CountByUser(ctx context.Context, userID uuid.UUID, filter domain.TaskFilter) (int, error) {
	query, args := buildCountQuery(userID, filter)
	var count int
	if err := r.db.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("CountByUser: %w", err)
	}
	return count, nil
}

// ── Dynamic query builder ─────────────────────────────────────────────────────

// buildListQuery constructs the SELECT + WHERE + ORDER + LIMIT for ListByUser.
// Uses positional parameters ($1, $2 ...) as required by pgx.
func buildListQuery(userID uuid.UUID, f domain.TaskFilter) (string, []any) {
	args := []any{userID}
	conds := []string{"user_id = $1"}
	n := 2 // next parameter index

	if len(f.Types) > 0 {
		conds = append(conds, fmt.Sprintf("type = ANY($%d)", n))
		args = append(args, typesToStrings(f.Types))
		n++
	}

	if len(f.Statuses) > 0 {
		conds = append(conds, fmt.Sprintf("status = ANY($%d)", n))
		args = append(args, statusesToStrings(f.Statuses))
		n++
	}

	if len(f.Labels) > 0 {
		// tasks must contain ALL specified labels
		conds = append(conds, fmt.Sprintf("labels @> $%d", n))
		args = append(args, f.Labels)
		n++
	}

	if f.From != nil {
		conds = append(conds, fmt.Sprintf("start_at >= $%d", n))
		args = append(args, f.From)
		n++
	}

	if f.To != nil {
		conds = append(conds, fmt.Sprintf("start_at <= $%d", n))
		args = append(args, f.To)
		n++
	}

	if f.ParentID != nil {
		conds = append(conds, fmt.Sprintf("parent_id = $%d", n))
		args = append(args, f.ParentID)
		n++
	}

	if f.OnlyRoots {
		conds = append(conds, "parent_id IS NULL")
	}

	// Archived handling
	if f.Archived == nil {
		// Default: exclude archived
		conds = append(conds, "archived = FALSE")
	} else if !*f.Archived {
		conds = append(conds, "archived = FALSE")
	}
	// *f.Archived == true → no filter, include all (both archived and not)

	if f.Pinned != nil {
		conds = append(conds, fmt.Sprintf("pinned = $%d", n))
		args = append(args, *f.Pinned)
		n++
	}

	if f.Search != "" {
		conds = append(conds, fmt.Sprintf(
			"(title ILIKE $%d OR details ILIKE $%d)", n, n,
		))
		args = append(args, "%"+f.Search+"%")
		n++
	}

	// Cursor: (created_at, id) < (cursor_time, cursor_id) for DESC ordering
	if f.Cursor != "" {
		cursorTime, cursorID, ok := parseCursorValues(f.Cursor)
		if ok {
			conds = append(conds, fmt.Sprintf(
				"(created_at, id) < ($%d, $%d)", n, n+1,
			))
			args = append(args, cursorTime, cursorID)
			n += 2
		}
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 20
	}
	// Fetch one extra to detect if there is a next page
	args = append(args, limit+1)

	where := strings.Join(conds, " AND ")
	query := fmt.Sprintf(`
		SELECT
			id, user_id, parent_id, type, title, details,
			status, priority, pinned, archived,
			occurrence_type, start_at, end_at, recurrence_rule,
			location, labels, created_at, updated_at
		FROM tasks
		WHERE %s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d
	`, where, n)

	return query, args
}

func buildCountQuery(userID uuid.UUID, f domain.TaskFilter) (string, []any) {
	args := []any{userID}
	conds := []string{"user_id = $1"}
	n := 2

	if len(f.Types) > 0 {
		conds = append(conds, fmt.Sprintf("type = ANY($%d)", n))
		args = append(args, typesToStrings(f.Types))
		n++
	}
	if len(f.Statuses) > 0 {
		conds = append(conds, fmt.Sprintf("status = ANY($%d)", n))
		args = append(args, statusesToStrings(f.Statuses))
		n++
	}
	if f.Archived == nil || !*f.Archived {
		conds = append(conds, "archived = FALSE")
	}

	_ = n
	where := strings.Join(conds, " AND ")
	return fmt.Sprintf("SELECT COUNT(*) FROM tasks WHERE %s", where), args
}

// ── Relations (links + attachments) ──────────────────────────────────────────

// hydrateRelations loads task_links and task_attachments for a task.
// Called after scanning the main task row.
func (r *postgresTaskRepository) hydrateRelations(ctx context.Context, task *domain.Task) error {
	links, err := r.loadLinks(ctx, task.ID)
	if err != nil {
		return err
	}
	task.Links = links

	attachments, err := r.loadAttachments(ctx, task.ID)
	if err != nil {
		return err
	}
	task.Attachments = attachments
	return nil
}

func (r *postgresTaskRepository) loadLinks(ctx context.Context, taskID uuid.UUID) ([]domain.TaskLink, error) {
	rows, err := r.db.Query(ctx, `
		SELECT label, url FROM task_links WHERE task_id = $1 ORDER BY id ASC
	`, taskID)
	if err != nil {
		return nil, fmt.Errorf("loadLinks: %w", err)
	}
	defer rows.Close()

	var links []domain.TaskLink
	for rows.Next() {
		var l domain.TaskLink
		if err := rows.Scan(&l.Label, &l.URL); err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	if links == nil {
		links = []domain.TaskLink{}
	}
	return links, rows.Err()
}

func (r *postgresTaskRepository) loadAttachments(ctx context.Context, taskID uuid.UUID) ([]domain.TaskAttachmentRef, error) {
	rows, err := r.db.Query(ctx, `
		SELECT storage_key, filename, content_type, size_bytes
		FROM task_attachments
		WHERE task_id = $1
		ORDER BY created_at ASC
	`, taskID)
	if err != nil {
		return nil, fmt.Errorf("loadAttachments: %w", err)
	}
	defer rows.Close()

	var attachments []domain.TaskAttachmentRef
	for rows.Next() {
		var a domain.TaskAttachmentRef
		if err := rows.Scan(&a.StorageKey, &a.Filename, &a.MimeType, &a.SizeBytes); err != nil {
			return nil, err
		}
		attachments = append(attachments, a)
	}
	if attachments == nil {
		attachments = []domain.TaskAttachmentRef{}
	}
	return attachments, rows.Err()
}

// saveRelations inserts task_links and task_attachments for a task.
func (r *postgresTaskRepository) saveRelations(ctx context.Context, task *domain.Task) error {
	for _, l := range task.Links {
		if _, err := r.db.Exec(ctx, `
			INSERT INTO task_links (id, task_id, label, url)
			VALUES ($1, $2, $3, $4)
		`, uuid.New(), task.ID, l.Label, l.URL); err != nil {
			return fmt.Errorf("saveLink: %w", err)
		}
	}

	for _, a := range task.Attachments {
		if _, err := r.db.Exec(ctx, `
			INSERT INTO task_attachments (id, task_id, storage_key, filename, content_type, size_bytes)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, uuid.New(), task.ID, a.StorageKey, a.Filename, a.MimeType, a.SizeBytes); err != nil {
			return fmt.Errorf("saveAttachment: %w", err)
		}
	}
	return nil
}

// deleteRelations removes all links and attachments for a task before re-inserting.
// Used by Update — simpler than diffing the collections.
func (r *postgresTaskRepository) deleteRelations(ctx context.Context, taskID uuid.UUID) error {
	if _, err := r.db.Exec(ctx, `DELETE FROM task_links WHERE task_id = $1`, taskID); err != nil {
		return fmt.Errorf("deleteLinks: %w", err)
	}
	if _, err := r.db.Exec(ctx, `DELETE FROM task_attachments WHERE task_id = $1`, taskID); err != nil {
		return fmt.Errorf("deleteAttachments: %w", err)
	}
	return nil
}

// ── Scan helpers ──────────────────────────────────────────────────────────────

// scanner is defined in scan.go and satisfied by both pgx.Row and pgx.Rows.

func scanTask(s scanner) (*domain.Task, error) {
	var (
		t              domain.Task
		parentID       *uuid.UUID
		statusStr      *string
		priorityStr    *string
		recurrenceJSON []byte
		startAt        *time.Time
		endAt          *time.Time
		occurrenceStr  string
		typeStr        string
	)

	err := s.Scan(
		&t.ID, &t.UserID, &parentID, &typeStr, &t.Title, &t.Details,
		&statusStr, &priorityStr, &t.Pinned, &t.Archived,
		&occurrenceStr, &startAt, &endAt, &recurrenceJSON,
		&t.Location, &t.Labels, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	t.ParentID = parentID
	t.Type = domain.TaskType(typeStr)
	t.OccurrenceType = domain.OccurrenceType(occurrenceStr)
	t.StartAt = startAt
	t.EndAt = endAt

	if statusStr != nil {
		s := domain.Status(*statusStr)
		t.Status = &s
	}
	if priorityStr != nil {
		p := domain.Priority(*priorityStr)
		t.Priority = &p
	}

	if recurrenceJSON != nil {
		rule, err := unmarshalRecurrenceRule(recurrenceJSON)
		if err != nil {
			return nil, fmt.Errorf("unmarshal recurrence rule: %w", err)
		}
		t.RecurrenceRule = rule
	}

	// Ensure slices are never nil (avoids null in JSON responses)
	if t.Labels == nil {
		t.Labels = []string{}
	}

	return &t, nil
}

// ── RecurrenceRule JSON marshalling ───────────────────────────────────────────
// RecurrenceRule is stored as JSONB in PostgreSQL.
// We use an intermediate struct for marshalling to keep the domain
// value object unexported-field-safe.

// recurrenceRuleJSON is the on-disk representation stored as JSONB.
// We use int for Weekday/Month so the values survive round-trips without
// depending on time.Weekday.String() formatting.
type recurrenceRuleJSON struct {
	Frequency  string     `json:"frequency"`
	Interval   int        `json:"interval"`
	ByWeekday  []int      `json:"by_weekday,omitempty"`   // time.Weekday values (0=Sunday)
	ByMonthDay []int      `json:"by_month_day,omitempty"`
	ByMonth    []int      `json:"by_month,omitempty"`     // time.Month values (1=January)
	BySetPos   []int      `json:"by_set_pos,omitempty"`
	Until      *time.Time `json:"until,omitempty"`
}

func marshalRecurrenceRule(rule *domain.RecurrenceRule) ([]byte, error) {
	if rule == nil {
		return nil, nil
	}

	days := make([]int, len(rule.ByWeekday))
	for i, d := range rule.ByWeekday {
		days[i] = int(d)
	}

	months := make([]int, len(rule.ByMonth))
	for i, m := range rule.ByMonth {
		months[i] = int(m)
	}

	return json.Marshal(recurrenceRuleJSON{
		Frequency:  string(rule.Frequency),
		Interval:   rule.Interval,
		ByWeekday:  days,
		ByMonthDay: rule.ByMonthDay,
		ByMonth:    months,
		BySetPos:   rule.BySetPos,
		Until:      rule.Until,
	})
}

func unmarshalRecurrenceRule(data []byte) (*domain.RecurrenceRule, error) {
	var r recurrenceRuleJSON
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}

	weekdays := make([]time.Weekday, len(r.ByWeekday))
	for i, d := range r.ByWeekday {
		weekdays[i] = time.Weekday(d)
	}

	months := make([]time.Month, len(r.ByMonth))
	for i, m := range r.ByMonth {
		months[i] = time.Month(m)
	}

	return &domain.RecurrenceRule{
		Frequency:  domain.RecurrenceType(r.Frequency),
		Interval:   r.Interval,
		ByWeekday:  weekdays,
		ByMonthDay: r.ByMonthDay,
		ByMonth:    months,
		BySetPos:   r.BySetPos,
		Until:      r.Until,
	}, nil
}

// ── Cursor helpers ────────────────────────────────────────────────────────────

func parseCursorValues(cursor string) (time.Time, uuid.UUID, bool) {
	// Cursor format: "2006-01-02T15:04:05.999999999Z:uuid"
	lastColon := strings.LastIndex(cursor, ":")
	if lastColon == -1 {
		return time.Time{}, uuid.Nil, false
	}
	timeStr := cursor[:lastColon]
	idStr := cursor[lastColon+1:]

	t, err := time.Parse(time.RFC3339Nano, timeStr)
	if err != nil {
		return time.Time{}, uuid.Nil, false
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return time.Time{}, uuid.Nil, false
	}
	return t, id, true
}

// ── Type conversion helpers ───────────────────────────────────────────────────

func statusPtr(s *domain.Status) *string {
	if s == nil {
		return nil
	}
	str := string(*s)
	return &str
}

func priorityPtr(p *domain.Priority) *string {
	if p == nil {
		return nil
	}
	str := string(*p)
	return &str
}

func typesToStrings(types []domain.TaskType) []string {
	out := make([]string, len(types))
	for i, t := range types {
		out[i] = string(t)
	}
	return out
}

func statusesToStrings(statuses []domain.Status) []string {
	out := make([]string, len(statuses))
	for i, s := range statuses {
		out[i] = string(s)
	}
	return out
}
