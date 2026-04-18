package logger

import (
	"io"
	"log/slog"
)

func NewLogger(logLevel string, w io.Writer) *slog.Logger {
	var handler slog.Handler
	var level slog.Level

	if err := level.UnmarshalText([]byte(logLevel)); err != nil {
		slog.Warn("could not parse log level. LevelError will be used", "level", logLevel, "error", err)
		level = slog.LevelError
	}

	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}

	handler = slog.NewTextHandler(w, opts)

	return slog.New(handler).With(
		slog.String("service", "scrapper-bot"),
	)
}
