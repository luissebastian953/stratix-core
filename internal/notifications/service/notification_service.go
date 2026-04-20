package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/luissebastian953/stratix-core/internal/domain"
	"github.com/luissebastian953/stratix-core/pkg/apperror"
)

// NotificationService manages device tokens and dispatches push notifications.
// It subscribes to domain events in bootstrap — no other module imports it.
type NotificationService struct {
	devices       domain.DeviceRepository
	notifications domain.NotificationRepository
	push          PushSender
}

func NewNotificationService(
	devices domain.DeviceRepository,
	notifications domain.NotificationRepository,
	push PushSender,
) *NotificationService {
	return &NotificationService{devices: devices, notifications: notifications, push: push}
}

// ── Device management ─────────────────────────────────────────────────────────

func (s *NotificationService) RegisterDevice(ctx context.Context, userID uuid.UUID, deviceID, token, platform string) error {
	if deviceID == "" {
		return apperror.BadRequest("device_id is required")
	}
	if token == "" {
		return apperror.BadRequest("token is required")
	}
	if platform != "ios" && platform != "android" {
		return apperror.BadRequest("platform must be ios or android")
	}

	device := &domain.Device{
		DeviceID:  deviceID,
		UserID:    userID,
		Token:     token,
		Platform:  platform,
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.devices.Upsert(ctx, device); err != nil {
		return apperror.Internal(err)
	}
	return nil
}

func (s *NotificationService) UnregisterDevice(ctx context.Context, userID uuid.UUID, deviceID string) error {
	// Fetch first to confirm ownership — don't expose whether a device exists.
	userDevices, err := s.devices.GetByUser(ctx, userID)
	if err != nil {
		return apperror.Internal(err)
	}
	for _, d := range userDevices {
		if d.DeviceID == deviceID {
			if err := s.devices.Delete(ctx, deviceID); err != nil {
				return apperror.Internal(err)
			}
			return nil
		}
	}
	return apperror.NotFound("device")
}

// ── Notification queries ──────────────────────────────────────────────────────

func (s *NotificationService) ListNotifications(ctx context.Context, userID uuid.UUID, limit int) ([]*domain.Notification, error) {
	ns, err := s.notifications.ListByUser(ctx, userID, limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return ns, nil
}

// ── Event handlers (called by Dispatcher) ────────────────────────────────────

func (s *NotificationService) OnTaskOverdue(ctx context.Context, evt domain.DomainEvent) error {
	e := evt.(domain.TaskOverdueEvent)
	return s.sendToUser(ctx, e.UserID, nil, "Task overdue", "A task you scheduled has passed its due date.")
}

// ── Internal helpers ──────────────────────────────────────────────────────────

// sendToUser delivers a push to every registered device for the user and
// records each sent notification in the DB.
func (s *NotificationService) sendToUser(ctx context.Context, userID uuid.UUID, taskID *uuid.UUID, title, body string) error {
	devices, err := s.devices.GetByUser(ctx, userID)
	if err != nil {
		return err
	}

	for _, d := range devices {
		if err := s.push.Send(ctx, d.Token, d.Platform, title, body); err != nil {
			slog.Error("push send failed",
				"device_id", d.DeviceID,
				"platform", d.Platform,
				"error", err,
			)
			continue
		}

		now := time.Now().UTC()
		n := &domain.Notification{
			ID:        uuid.New(),
			UserID:    userID,
			TaskID:    taskID,
			Channel:   d.Platform,
			Title:     title,
			Body:      body,
			SentAt:    now,
			CreatedAt: now,
		}
		if err := s.notifications.Save(ctx, n); err != nil {
			slog.Error("save notification failed", "error", err)
		}
	}

	return nil
}
