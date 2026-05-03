package http

import (
	"context"
	"errors"
	"fmt"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"log/slog"
	"net/http"
	"time"
)

type SubscriptionService interface {
	RegisterChat(ctx context.Context, chatID int64) error
	DeleteChat(ctx context.Context, chatID int64) error
	GetTrackedLinks(ctx context.Context, chatID int64) ([]domain.TrackedLink, error)
	AddLink(ctx context.Context, chatID int64, url string, tags []string) (domain.TrackedLink, error)
	RemoveLink(ctx context.Context, chatID int64, url string) (domain.TrackedLink, error)
}

type Server struct {
	httpServer *http.Server
}

func NewServer(port uint16, service SubscriptionService, logger *slog.Logger) *Server {
	handler := NewHandler(service, logger)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /tg-chat/{id}", handler.HandlePostTgChat)
	mux.HandleFunc("DELETE /tg-chat/{id}", handler.HandleDeleteTgChat)
	mux.HandleFunc("GET /links", handler.HandleGetLinks)
	mux.HandleFunc("POST /links", handler.HandlePostLinks)
	mux.HandleFunc("DELETE /links", handler.HandleDeleteLinks)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	return &Server{
		httpServer: server,
	}
}

func (server *Server) Start(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		if err := server.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("http server failed: %w", err)

	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("http server shutdown failed: %w", err)
		}

		return nil
	}
}
