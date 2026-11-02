package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"

	"conduit/backend/internal/config"
)

func New(cfg config.Config) (*slog.Logger, func() error, error) {
	if cfg.Logging.Silent {
		return slog.New(slog.DiscardHandler), func() error { return nil }, nil
	}

	level := parseLevel(cfg.Logging.Level)
	options := &slog.HandlerOptions{Level: level}

	var console slog.Handler
	if cfg.App.IsProduction {
		console = slog.NewJSONHandler(os.Stdout, options)
	} else {
		console = slog.NewTextHandler(os.Stdout, options)
	}
	handlers := []slog.Handler{console}
	closeFn := func() error { return nil }

	if cfg.Logging.ToFile {
		file, err := NewRotatingFile(RotatingFileConfig{
			Dir:        cfg.Logging.Dir,
			Prefix:     cfg.App.Name,
			MaxBytes:   20 * 1024 * 1024,
			MaxAgeDays: 14,
			Compress:   true,
		})
		if err != nil {
			return nil, nil, err
		}
		handlers = append(handlers, slog.NewJSONHandler(io.Writer(file), options))
		closeFn = file.Close
	}

	logger := slog.New(slog.NewMultiHandler(handlers...)).With("service", cfg.App.Name)
	return logger, closeFn, nil
}

func parseLevel(value string) slog.Level {
	switch strings.ToLower(value) {
	case "error":
		return slog.LevelError
	case "warn", "warning":
		return slog.LevelWarn
	case "info", "http":
		return slog.LevelInfo
	case "silly":
		return slog.LevelDebug - 4
	default:
		return slog.LevelDebug
	}
}
