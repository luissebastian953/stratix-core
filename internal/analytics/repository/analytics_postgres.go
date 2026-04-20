package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/luissebastian953/stratix-core/internal/domain"
)

type AnalyticsRepository struct {
	db *pgxpool.Pool
}

func NewAnalyticsRepository(db *pgxpool.Pool) domain.AnalyticsRepository {
	return &AnalyticsRepository{db: db}
}

func (r *AnalyticsRepository) Save(ctx context.Context, event *domain.AnalyticsEvent) error {
	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO analytics_events (id, user_id, event_type, payload, occurred_at)
		VALUES ($1, $2, $3, $4, $5)
	`, event.ID, event.UserID, event.EventType, payload, event.OccurredAt)
	return err
}

func (r *AnalyticsRepository) Summary(ctx context.Context, userID uuid.UUID) (*domain.TaskSummary, error) {
	var s domain.TaskSummary

	err := r.db.QueryRow(ctx, `
		SELECT
			COUNT(*)                                                          AS total,
			COUNT(*) FILTER (WHERE status = 'completed')                     AS completed,
			COUNT(*) FILTER (WHERE end_at < NOW() AND status != 'completed') AS overdue,
			COUNT(*) FILTER (WHERE archived = TRUE)                          AS archived
		FROM tasks
		WHERE user_id = $1
	`, userID).Scan(&s.Total, &s.Completed, &s.Overdue, &s.Archived)
	if err != nil {
		return nil, fmt.Errorf("Summary: %w", err)
	}

	return &s, nil
}

func (r *AnalyticsRepository) TypeDistribution(ctx context.Context, userID uuid.UUID) ([]domain.TypeCount, error) {
	rows, err := r.db.Query(ctx, `
		SELECT type, COUNT(*) AS count
		FROM tasks
		WHERE user_id = $1 AND archived = FALSE
		GROUP BY type
		ORDER BY count DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("TypeDistribution: %w", err)
	}
	defer rows.Close()

	var out []domain.TypeCount
	for rows.Next() {
		var tc domain.TypeCount
		if err := rows.Scan(&tc.Type, &tc.Count); err != nil {
			return nil, err
		}
		out = append(out, tc)
	}
	if out == nil {
		out = []domain.TypeCount{}
	}
	return out, rows.Err()
}

func (r *AnalyticsRepository) CompletedPerDay(ctx context.Context, userID uuid.UUID, days int) (map[string]int, error) {
	since := time.Now().UTC().AddDate(0, 0, -days)

	rows, err := r.db.Query(ctx, `
		SELECT occurred_at::date AS day, COUNT(*) AS count
		FROM analytics_events
		WHERE user_id    = $1
		  AND event_type = 'task.completed'
		  AND occurred_at >= $2
		GROUP BY day
		ORDER BY day ASC
	`, userID, since)
	if err != nil {
		return nil, fmt.Errorf("CompletedPerDay: %w", err)
	}
	defer rows.Close()

	out := make(map[string]int)
	for rows.Next() {
		var day time.Time
		var count int
		if err := rows.Scan(&day, &count); err != nil {
			return nil, err
		}
		out[day.Format("2006-01-02")] = count
	}
	return out, rows.Err()
}
