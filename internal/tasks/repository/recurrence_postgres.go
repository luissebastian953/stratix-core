package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/luissebastian953/stratix-core/internal/domain"
)

// postgresRecurrenceRepository implements domain.RecurrenceRepository.
// Instances are persisted only when acted on (completed or skipped) —
// pending instances are generated in-memory by RecurrenceService and
// are not written to the database until the user acts on them.
type postgresRecurrenceRepository struct {
	db *pgxpool.Pool
}

func NewRecurrenceRepository(db *pgxpool.Pool) domain.RecurrenceRepository {
	return &postgresRecurrenceRepository{db: db}
}

// ── domain.RecurrenceRepository ───────────────────────────────────────────────

func (r *postgresRecurrenceRepository) ListInstancesInRange(
	ctx context.Context,
	taskID uuid.UUID,
	from, to time.Time,
) ([]*domain.RecurrenceInstance, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, task_id, scheduled_at, status, completed_at
		FROM recurrence_instances
		WHERE task_id      = $1
		  AND scheduled_at >= $2
		  AND scheduled_at <  $3
		ORDER BY scheduled_at ASC
	`, taskID, from.UTC(), to.UTC())
	if err != nil {
		return nil, fmt.Errorf("ListInstancesInRange: %w", err)
	}
	defer rows.Close()

	var instances []*domain.RecurrenceInstance
	for rows.Next() {
		inst, err := scanInstance(rows)
		if err != nil {
			return nil, fmt.Errorf("ListInstancesInRange scan: %w", err)
		}
		instances = append(instances, inst)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ListInstancesInRange rows: %w", err)
	}
	if instances == nil {
		instances = []*domain.RecurrenceInstance{}
	}
	return instances, nil
}

func (r *postgresRecurrenceRepository) GetInstance(ctx context.Context, id uuid.UUID) (*domain.RecurrenceInstance, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, task_id, scheduled_at, status, completed_at
		FROM recurrence_instances
		WHERE id = $1
		LIMIT 1
	`, id)

	inst, err := scanInstance(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.NotFoundError{Resource: "recurrence_instance", ID: id.String()}
		}
		return nil, fmt.Errorf("GetInstance: %w", err)
	}
	return inst, nil
}

func (r *postgresRecurrenceRepository) SaveInstance(ctx context.Context, instance *domain.RecurrenceInstance) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO recurrence_instances (id, task_id, scheduled_at, status, completed_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (task_id, scheduled_at) DO NOTHING
	`,
		instance.ID,
		instance.TaskID,
		instance.ScheduledAt.UTC(),
		string(instance.Status),
		instance.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("SaveInstance: %w", err)
	}
	return nil
}

func (r *postgresRecurrenceRepository) UpdateInstance(ctx context.Context, instance *domain.RecurrenceInstance) error {
	result, err := r.db.Exec(ctx, `
		UPDATE recurrence_instances
		SET status       = $2,
		    completed_at = $3
		WHERE id = $1
	`,
		instance.ID,
		string(instance.Status),
		instance.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("UpdateInstance: %w", err)
	}
	if result.RowsAffected() == 0 {
		return &domain.NotFoundError{Resource: "recurrence_instance", ID: instance.ID.String()}
	}
	return nil
}

func (r *postgresRecurrenceRepository) DeleteByTaskID(ctx context.Context, taskID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM recurrence_instances WHERE task_id = $1
	`, taskID)
	if err != nil {
		return fmt.Errorf("DeleteByTaskID: %w", err)
	}
	return nil
}

// ListPendingOverdue returns pending instances whose scheduled_at is in the past.
// Used by the background scheduler to emit TaskOverdueEvents.
// limit caps the batch size so one scheduler tick doesn't process thousands of rows.
func (r *postgresRecurrenceRepository) ListPendingOverdue(ctx context.Context, limit int) ([]*domain.RecurrenceInstance, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, task_id, scheduled_at, status, completed_at
		FROM recurrence_instances
		WHERE status       = 'pending'
		  AND scheduled_at < NOW()
		ORDER BY scheduled_at ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("ListPendingOverdue: %w", err)
	}
	defer rows.Close()

	var instances []*domain.RecurrenceInstance
	for rows.Next() {
		inst, err := scanInstance(rows)
		if err != nil {
			return nil, fmt.Errorf("ListPendingOverdue scan: %w", err)
		}
		instances = append(instances, inst)
	}
	if instances == nil {
		instances = []*domain.RecurrenceInstance{}
	}
	return instances, rows.Err()
}

// ── Scan helper ───────────────────────────────────────────────────────────────

func scanInstance(s scanner) (*domain.RecurrenceInstance, error) {
	var (
		inst        domain.RecurrenceInstance
		statusStr   string
		completedAt *time.Time
	)
	err := s.Scan(
		&inst.ID,
		&inst.TaskID,
		&inst.ScheduledAt,
		&statusStr,
		&completedAt,
	)
	if err != nil {
		return nil, err
	}
	inst.Status = domain.InstanceStatus(statusStr)
	inst.CompletedAt = completedAt
	inst.ScheduledAt = inst.ScheduledAt.UTC()
	return &inst, nil
}
