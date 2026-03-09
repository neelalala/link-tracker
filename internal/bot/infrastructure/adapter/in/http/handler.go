package http

import (
	"encoding/json"
	"errors"
	scrapperdomain "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"io"
	"log/slog"
	"net/http"
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
	TgChatIds   []int64 `json:"tgChatIds"`
}
type apiErrorResponse struct {
	Description      string   `json:"description"`
	Code             string   `json:"code"`
	ExceptionName    string   `json:"exceptionName"`
	ExceptionMessage string   `json:"exceptionMessage"`
	Stacktrace       []string `json:"stacktrace"`
}

func (handler *Handler) HandleUpdates(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, BodyBytesLimit))
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

		resp := apiErrorResponse{
			Description:      "Error reading request body",
			Code:             "internal_error",
			ExceptionName:    "body_request_reading_exception",
			ExceptionMessage: "",
			Stacktrace:       []string{"internal_error"},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(resp)
		return
	}
	defer r.Body.Close()

	var req updateRequest
	err = json.Unmarshal(body, &req)
	if err != nil {
		handler.logger.Warn(
			"Bad request body",
			slog.String("context", "handler.HandleUpdates"),
			slog.String("error", err.Error()),
			slog.String("request_body", string(body)),
		)

		resp := apiErrorResponse{
			Description:      "Bad request parameters",
			Code:             "bad_request",
			ExceptionName:    "bad_request_parameters",
			ExceptionMessage: "Could not parse request body. Body: " + string(body),
			Stacktrace:       []string{err.Error()},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(resp)
		return
	}

	linkUpdate := scrapperdomain.LinkUpdate{
		ID:          req.Id,
		URL:         req.Url,
		Description: req.Description,
		TgChatIDs:   req.TgChatIds,
	}

	err = handler.updateHandler.HandleUpdate(linkUpdate)
	if err != nil {
		handler.logger.Warn(
			"Error while handling update on link",
			slog.String("context", "handler.updateHandler.HandleUpdate"),
			slog.String("error", err.Error()),
			slog.String("link", req.Url),
		)

		resp := apiErrorResponse{
			Description:      "Error handling update on link",
			Code:             "internal_error",
			ExceptionName:    "update_handler_exception",
			ExceptionMessage: "Could not handle update on link",
			Stacktrace:       []string{err.Error()},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(resp)
		return
	}

	w.WriteHeader(http.StatusOK)
}
