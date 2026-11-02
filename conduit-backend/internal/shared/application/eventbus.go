package application

import (
	"context"

	"conduit/backend/internal/shared/domain"
)

type EventHandler func(ctx context.Context, event domain.Event) error

type EventBus interface {
	Publish(ctx context.Context, event domain.Event)
	Subscribe(eventName string, handler EventHandler)
}
