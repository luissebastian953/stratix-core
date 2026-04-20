package middleware

import (
	"sync"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/luissebastian953/stratix-core/pkg/apperror"
)

type userBucket struct {
	count     int
	windowEnd time.Time
}

type aiRateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*userBucket
	requests int
	window   time.Duration
}

// AIRateLimit limits AI endpoint calls per user.
// Must be applied after the JWT middleware so UserIDKey is populated.
func AIRateLimit(requests int, window time.Duration) echo.MiddlewareFunc {
	limiter := &aiRateLimiter{
		buckets:  make(map[string]*userBucket),
		requests: requests,
		window:   window,
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID, ok := c.Get(UserIDKey).(string)
			if !ok || userID == "" {
				return apperror.Unauthorized("missing user identity")
			}

			if !limiter.allow(userID) {
				return apperror.TooManyRequests("AI rate limit exceeded — try again later")
			}

			return next(c)
		}
	}
}

func (l *aiRateLimiter) allow(userID string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	b, ok := l.buckets[userID]
	if !ok || now.After(b.windowEnd) {
		l.buckets[userID] = &userBucket{count: 1, windowEnd: now.Add(l.window)}
		return true
	}

	if b.count >= l.requests {
		return false
	}

	b.count++
	return true
}
