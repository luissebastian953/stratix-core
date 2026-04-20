package events

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/luissebastian953/stratix-core/internal/domain"
)

type Dispatcher struct {
	mu       sync.RWMutex
	handlers map[string][]domain.HandlerFunc
}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{handlers: make(map[string][]domain.HandlerFunc)}
}

func (d *Dispatcher) Register(eventName string, h domain.HandlerFunc) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers[eventName] = append(d.handlers[eventName], h)
}

func (d *Dispatcher) Dispatch(ctx context.Context, event domain.DomainEvent) {
	d.mu.RLock()
	handlers := make([]domain.HandlerFunc, len(d.handlers[event.EventName()]))
	copy(handlers, d.handlers[event.EventName()])
	d.mu.RUnlock()

	for _, h := range handlers {
		h := h
		go func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("event handler panic",
						"event", event.EventName(),
						"recover", r,
					)
				}
			}()

			handlerCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if err := h(handlerCtx, event); err != nil {
				slog.Error("event handler error",
					"event", event.EventName(),
					"error", err,
				)
			}
		}()
	}
}
