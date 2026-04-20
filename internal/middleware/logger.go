package middleware

import (
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func Logger() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			err := next(c)

			req := c.Request()
			res := c.Response()

			var userID string

			if id, ok := c.Get(UserIDKey).(uuid.UUID); ok && id != uuid.Nil {
				userID = id.String()
			}

			status := res.Status
			logFn := slog.Info

			if status >= 500 {
				logFn = slog.Error
			} else if status >= 400 {
				logFn = slog.Warn
			}

			logFn("request",
				"method", req.Method,
				"path", req.URL.Path,
				"status", status,
				"latency_ms", time.Since(start).Milliseconds(),
				"user_id", userID,
				"ip", c.RealIP(),
				"request_id", res.Header().Get(echo.HeaderXRequestID),
				"bytes_out", res.Size,
			)

			return err
		}
	}
}
