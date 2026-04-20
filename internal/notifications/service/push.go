package service

import (
	"context"
	"log/slog"
)

// PushSender abstracts FCM (Android) and APNs (iOS) behind a single interface.
// Swap the stub below for real implementations once credentials are available.
type PushSender interface {
	Send(ctx context.Context, token, platform, title, body string) error
}

// StubPushSender logs instead of sending — safe for development.
type StubPushSender struct{}

func NewStubPushSender() PushSender {
	return &StubPushSender{}
}

func (s *StubPushSender) Send(_ context.Context, token, platform, title, body string) error {
	slog.Info("push notification (stub)",
		"platform", platform,
		"token", token[:min(8, len(token))]+"...",
		"title", title,
		"body", body,
	)
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
