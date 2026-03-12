package http

import (
	"context"
	"errors"
	"fmt"
	scrapperdomain "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"log/slog"
	"net/http"
	"time"
)

type LinkUpdateHandler interface {
	HandleUpdate(ctx context.Context, update scrapperdomain.LinkUpdate) error
}

type Server struct {
	httpServer *http.Server
}

func NewServer(port uint16, updateHandler LinkUpdateHandler, logger *slog.Logger) *Server {
	handler := NewHandler(updateHandler, logger)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /updates", handler.HandleUpdates)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	return &Server{
		httpServer: srv,
	}
}

func (s *Server) Start(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("bot http server failed: %w", err)

	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("bot http server shutdown failed: %w", err)
		}

		return nil
	}
}
