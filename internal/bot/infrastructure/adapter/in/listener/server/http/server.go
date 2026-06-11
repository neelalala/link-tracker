package http

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/resilience"
)

type Server struct {
	httpServer *http.Server
	rl         *resilience.IPRateLimiter
	log        *slog.Logger
}

func NewServer(port uint16, updateHandler domain.LinkUpdateHandler, rateLimitConfig resilience.RateLimitConfig, log *slog.Logger) *Server {
	handler := NewHandler(updateHandler, log)

	rl := resilience.NewIPRateLimiter(rateLimitConfig, log)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /updates", rl.Middleware(handler.HandleUpdates))

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	return &Server{
		httpServer: server,
		rl:         rl,
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
	server.rl.Stop()
	return server.httpServer.Shutdown(ctx)
}
