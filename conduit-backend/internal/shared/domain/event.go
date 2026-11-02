package domain

import "time"

const ArticleDeletedEvent = "article.deleted"

type ArticleDeleted struct {
	ArticleID string
}

type Event struct {
	Name       string
	Payload    any
	OccurredAt time.Time
}

func NewEvent(name string, payload any, occurredAt time.Time) Event {
	return Event{Name: name, Payload: payload, OccurredAt: occurredAt.UTC()}
}
