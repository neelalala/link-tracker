package http

import (
	"context"
	"errors"
	"fmt"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
	"log/slog"
	"net/http"
)

type Server struct {
	httpServer *http.Server
	log        *slog.Logger
}

func NewServer(port uint16, updateHandler domain.LinkUpdateHandler, log *slog.Logger) *Server {
	handler := NewHandler(updateHandler, log)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /updates", handler.HandleUpdates)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	return &Server{
		httpServer: server,
		log:        log,
	}
}

func (server *Server) Start() error {
	server.log.Info("HTTP server is running")
	if err := server.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (server *Server) Stop(ctx context.Context) error {
	server.log.Info("Shutting down HTTP server...")
	return server.httpServer.Shutdown(ctx)
}
