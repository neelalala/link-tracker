package http

import (
	"context"
	"errors"
	"fmt"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"log/slog"
	"net/http"
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
	port       uint16
	log        *slog.Logger
}

func NewServer(port uint16, service SubscriptionService, log *slog.Logger) *Server {
	handler := NewHandler(service, log)
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
		port:       port,
		log:        log,
	}
}

func (server *Server) Start() error {
	server.log.Info("HTTP server is running", slog.Int("port", int(server.port)))
	if err := server.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (server *Server) Stop(ctx context.Context) error {
	server.log.Info("Shutting down HTTP server...")
	return server.httpServer.Shutdown(ctx)
}
