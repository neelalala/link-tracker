package http

import (
	"fmt"
	scrapperdomain "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"log/slog"
	"net/http"
)

type LinkUpdateHandler interface {
	HandleUpdate(update scrapperdomain.LinkUpdate) error
}

type Server struct {
	port uint16
	mux  *http.ServeMux
}

func NewServer(port uint16, updateHandler LinkUpdateHandler, logger *slog.Logger) *Server {
	handler := NewHandler(updateHandler, logger)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /updates", handler.HandleUpdates)

	return &Server{
		port: port,
		mux:  mux,
	}
}

func (s *Server) Start() error {
	return http.ListenAndServe(fmt.Sprintf(":%d", s.port), s.mux)
}
