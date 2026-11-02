package bootstrap

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/labstack/echo/v5"

	"conduit/backend/internal/config"
)

type Server struct {
	Container *Container
	Echo      *echo.Echo
	Addr      net.Addr

	cancel   context.CancelFunc
	done     chan struct{}
	serveErr error
	stopOnce sync.Once
	stopErr  error
}

func Start(ctx context.Context, cfg config.Config, logger *slog.Logger) (*Server, error) {
	container, err := NewContainer(ctx, cfg, logger)
	if err != nil {
		return nil, err
	}

	app, err := NewApp(container)
	if err != nil {
		container.Close()
		return nil, err
	}

	listener, err := net.Listen("tcp", net.JoinHostPort(cfg.App.Host, strconv.Itoa(cfg.App.Port)))
	if err != nil {
		container.Close()
		return nil, err
	}

	serveCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	server := &Server{
		Container: container,
		Echo:      app,
		Addr:      listener.Addr(),
		cancel:    cancel,
		done:      make(chan struct{}),
	}

	startConfig := echo.StartConfig{
		Listener:        listener,
		HideBanner:      true,
		HidePort:        true,
		GracefulTimeout: 10 * time.Second,
		BeforeServeFunc: func(s *http.Server) error {
			s.ReadHeaderTimeout = 10 * time.Second
			s.IdleTimeout = 120 * time.Second
			return nil
		},
	}
	go func() {
		server.serveErr = startConfig.Start(serveCtx, app)
		close(server.done)
	}()

	logger.Info("Server is running on http://" + listener.Addr().String())
	return server, nil
}

func (s *Server) Port() int {
	if tcp, ok := s.Addr.(*net.TCPAddr); ok {
		return tcp.Port
	}
	return 0
}

func (s *Server) Done() <-chan struct{} {
	return s.done
}

func (s *Server) Err() error {
	<-s.done
	return s.serveErr
}

func (s *Server) Stop() error {
	s.stopOnce.Do(func() {
		s.cancel()
		<-s.done
		s.stopErr = s.serveErr
		s.Container.Close()
	})
	return s.stopErr
}
