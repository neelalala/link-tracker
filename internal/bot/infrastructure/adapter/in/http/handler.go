package http

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

const BodyBytesLimit = 1 << 20

type Handler struct {
	updateHandler LinkUpdateHandler
	logger        *slog.Logger
}

func NewHandler(updateHandler LinkUpdateHandler, logger *slog.Logger) *Handler {
	return &Handler{
		updateHandler: updateHandler,
		logger:        logger,
	}
}

type updateRequest struct {
	Id          int64   `json:"id"`
	Url         string  `json:"url"`
	Description string  `json:"description"`
	Preview     string  `json:"preview"`
	TgChatIds   []int64 `json:"tgChatIds"`
}

func (handler *Handler) HandleUpdates(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, BodyBytesLimit))
	defer r.Body.Close()
	if err != nil {
		if errors.Is(err, io.EOF) {
			handler.logger.Warn("Request body EOF. Probably body size > 1 MB", slog.String("context", "handler.HandleUpdates"), slog.String("error", err.Error()))
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Request body too large"))
			return
		}
		handler.logger.Error(
			"Error reading request body",
			slog.String("context", "handler.HandleUpdates"),
			slog.String("error", err.Error()),
		)

		handler.writeError(w, http.StatusInternalServerError,
			"Error reading request body",
			"internal_error",
			"body_request_reading_exception",
			"Error while reading request body",
			err,
		)
		return
	}

	var request updateRequest
	err = json.Unmarshal(body, &request)
	if err != nil {
		handler.logger.Warn(
			"Bad request body",
			slog.String("context", "handler.HandleUpdates"),
			slog.String("error", err.Error()),
			slog.String("request_body", string(body)),
		)

		handler.writeError(w, http.StatusBadRequest,
			"Bad request parameters",
			"bad_request",
			"bad_request_parameters",
			"Could not parse request body. Body: "+string(body),
			err,
		)
		return
	}

	linkUpdate := domain.LinkUpdate{
		ID:          request.Id,
		URL:         request.Url,
		Description: request.Description,
		Preview:     request.Preview,
		TgChatIDs:   request.TgChatIds,
	}

	ctx := r.Context()

	err = handler.updateHandler.HandleUpdate(ctx, linkUpdate)
	if err != nil {
		handler.logger.Warn(
			"Error while handling update on link",
			slog.String("context", "handler.updateHandler.HandleUpdate"),
			slog.String("error", err.Error()),
			slog.String("link", request.Url),
		)

		handler.writeError(w, http.StatusInternalServerError,
			"Error handling update on link",
			"internal_error",
			"update_handler_exception",
			"Could not handle update on link",
			err,
		)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (handler *Handler) writeError(w http.ResponseWriter, status int, desc, code, excName, excMessage string, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	type apiErrorResponse struct {
		Description      string   `json:"description"`
		Code             string   `json:"code"`
		ExceptionName    string   `json:"exceptionName"`
		ExceptionMessage string   `json:"exceptionMessage"`
		Stacktrace       []string `json:"stacktrace"`
	}

	resp := apiErrorResponse{
		Description:      desc,
		Code:             code,
		ExceptionName:    excName,
		ExceptionMessage: excMessage,
		Stacktrace:       []string{err.Error()},
	}
	_ = json.NewEncoder(w).Encode(resp)
}
