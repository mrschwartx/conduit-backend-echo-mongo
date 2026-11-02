package events

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"conduit/backend/internal/shared/domain"
)

func TestPublishDeliversToAllSubscribersInOrder(t *testing.T) {
	bus := NewInMemoryBus(slog.New(slog.DiscardHandler))
	calls := []string{}
	bus.Subscribe("a", func(context.Context, domain.Event) error { calls = append(calls, "first"); return nil })
	bus.Subscribe("a", func(context.Context, domain.Event) error { calls = append(calls, "second"); return nil })
	bus.Subscribe("b", func(context.Context, domain.Event) error { calls = append(calls, "other"); return nil })

	bus.Publish(context.Background(), domain.NewEvent("a", nil, time.Now()))

	if strings.Join(calls, ",") != "first,second" {
		t.Fatalf("calls = %v", calls)
	}
}

func TestPublishIsolatesFailingAndPanickingHandlers(t *testing.T) {
	var logs bytes.Buffer
	bus := NewInMemoryBus(slog.New(slog.NewTextHandler(&logs, nil)))
	reached := false
	bus.Subscribe("x", func(context.Context, domain.Event) error { return errors.New("boom") })
	bus.Subscribe("x", func(context.Context, domain.Event) error { panic("kaput") })
	bus.Subscribe("x", func(context.Context, domain.Event) error { reached = true; return nil })

	bus.Publish(context.Background(), domain.NewEvent("x", nil, time.Now()))

	if !reached {
		t.Fatal("later handlers must still run")
	}
	output := logs.String()
	if !strings.Contains(output, "boom") || !strings.Contains(output, "kaput") {
		t.Fatalf("failures were not logged: %s", output)
	}
}

func TestPublishWithoutSubscribersIsANoop(t *testing.T) {
	NewInMemoryBus(slog.New(slog.DiscardHandler)).Publish(context.Background(), domain.NewEvent("none", nil, time.Now()))
}
