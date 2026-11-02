package events

import (
	"context"
	"log/slog"
	"sync"

	"conduit/backend/internal/shared/application"
	"conduit/backend/internal/shared/domain"
)

type InMemoryBus struct {
	mu       sync.RWMutex
	handlers map[string][]application.EventHandler
	logger   *slog.Logger
}

func NewInMemoryBus(logger *slog.Logger) *InMemoryBus {
	return &InMemoryBus{
		handlers: make(map[string][]application.EventHandler),
		logger:   logger,
	}
}

func (bus *InMemoryBus) Subscribe(eventName string, handler application.EventHandler) {
	bus.mu.Lock()
	defer bus.mu.Unlock()
	bus.handlers[eventName] = append(bus.handlers[eventName], handler)
}

func (bus *InMemoryBus) Publish(ctx context.Context, event domain.Event) {
	bus.mu.RLock()
	handlers := append([]application.EventHandler(nil), bus.handlers[event.Name]...)
	bus.mu.RUnlock()

	for _, handler := range handlers {
		if err := bus.safeHandle(ctx, handler, event); err != nil {
			bus.logger.Error("Event handler failed", "event", event.Name, "message", err.Error())
		}
	}
}

func (bus *InMemoryBus) safeHandle(ctx context.Context, handler application.EventHandler, event domain.Event) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = panicError{value: recovered}
		}
	}()
	return handler(ctx, event)
}

type panicError struct {
	value any
}

func (e panicError) Error() string {
	if err, ok := e.value.(error); ok {
		return err.Error()
	}
	if text, ok := e.value.(string); ok {
		return text
	}
	return "handler panicked"
}
