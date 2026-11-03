package domain

import "time"

const timestampLayout = "2006-01-02T15:04:05.000Z"

func FormatTimestamp(value time.Time) string {
	return value.UTC().Format(timestampLayout)
}

func TruncateToMillisecond(value time.Time) time.Time {
	return value.UTC().Truncate(time.Millisecond)
}
