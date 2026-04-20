package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/luissebastian953/stratix-core/internal/domain"
)

type NotificationRepository struct {
	db *pgxpool.Pool
}

func NewNotificationRepository(db *pgxpool.Pool) domain.NotificationRepository {
	return &NotificationRepository{db: db}
}

func (r *NotificationRepository) Save(ctx context.Context, n *domain.Notification) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO notifications (id, user_id, task_id, channel, title, body, sent_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, n.ID, n.UserID, n.TaskID, n.Channel, n.Title, n.Body, n.SentAt, n.CreatedAt)
	if err != nil {
		return fmt.Errorf("Save notification: %w", err)
	}
	return nil
}

func (r *NotificationRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]*domain.Notification, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, task_id, channel, title, body, sent_at, created_at
		FROM notifications
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("ListByUser notifications: %w", err)
	}
	defer rows.Close()

	var notifications []*domain.Notification
	for rows.Next() {
		var n domain.Notification
		if err := rows.Scan(
			&n.ID, &n.UserID, &n.TaskID, &n.Channel,
			&n.Title, &n.Body, &n.SentAt, &n.CreatedAt,
		); err != nil {
			return nil, err
		}
		notifications = append(notifications, &n)
	}
	if notifications == nil {
		notifications = []*domain.Notification{}
	}
	return notifications, rows.Err()
}
