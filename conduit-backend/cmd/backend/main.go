package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"

	"conduit/backend/internal/bootstrap"
	"conduit/backend/internal/config"
	"conduit/backend/internal/shared/infrastructure/logging"
)

func main() {
	os.Exit(run())
}

func run() int {
	_ = godotenv.Load()

	cfg, err := config.Load(os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}

	logger, closeLogger, err := logging.New(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	defer closeLogger()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	server, err := bootstrap.Start(ctx, cfg, logger)
	if err != nil {
		logger.Error("Failed to start server", "message", err.Error())
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}

	select {
	case <-ctx.Done():
		logger.Info("Received shutdown signal, shutting down gracefully")
	case <-server.Done():
	}

	if err := server.Stop(); err != nil {
		logger.Error("Error during shutdown", "message", err.Error())
		return 1
	}
	logger.Info("Shutdown complete")
	return 0
}
