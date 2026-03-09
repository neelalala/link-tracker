package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"io"
	"log/slog"
	"net/http"
	"strconv"
)

type linkResponse struct {
	Id      int64    `json:"id"`
	Url     string   `json:"url"`
	Tags    []string `json:"tags"`
	Filters []string `json:"filters"`
}

type apiErrorResponse struct {
	Description      string   `json:"description"`
	Code             string   `json:"code"`
	ExceptionName    string   `json:"exceptionName"`
	ExceptionMessage string   `json:"exceptionMessage"`
	Stacktrace       []string `json:"stacktrace"`
}

type Handler struct {
	service application.SubscriptionService

	logger *slog.Logger
}

func NewHandler(service application.SubscriptionService, logger *slog.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

func (handler *Handler) getChatIDFromHeader(r *http.Request) (int64, error) {
	chatIdStr := r.Header.Get("Tg-Chat-Id")
	if chatIdStr == "" {
		return 0, errors.New("Tg-Chat-Id header is missing")
	}
	return strconv.ParseInt(chatIdStr, 10, 64)
}

func (handler *Handler) getChatIdFromPath(r *http.Request) (int64, error) {
	chatIdStr := r.PathValue("id")
	if chatIdStr == "" {
		return 0, errors.New("id path param is missing")
	}
	return strconv.ParseInt(chatIdStr, 10, 64)
}

func (handler *Handler) HandlePostTgChat(w http.ResponseWriter, r *http.Request) {
	chatId, err := handler.getChatIdFromPath(r)
	if err != nil {
		handler.logger.Warn(
			"failed to parse chat id from query string",
			slog.String("error", err.Error()),
			slog.String("context", "handler.HandlePostTgChat"),
		)
		handler.writeError(
			w, http.StatusBadRequest,
			"Error while parsing chat id as integer",
			"bad_request",
			"bad_query_request_exception",
			"Could not parse chat id",
			err,
		)
		return
	}

	err = handler.service.RegisterChat(chatId)
	if err != nil {
		if errors.Is(err, domain.ErrChatAlreadyRegistered) {
			handler.writeError(w, http.StatusConflict,
				"Chat already registered",
				"conflict",
				"bad_request_exception",
				fmt.Sprintf("Chat with id %d already registered", chatId),
				err,
			)
			return
		}

		handler.logger.Error(
			"failed to register chat",
			slog.Int64("chat_id", chatId),
			slog.String("error", err.Error()),
			slog.String("context", "handler.HandlePostTgChat"),
		)
		handler.writeError(w, http.StatusInternalServerError,
			"Error while saving chat",
			"internal_server_error",
			"save_tgchat_exception",
			fmt.Sprintf("Could not save chat with id: %d", chatId),
			err,
		)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (handler *Handler) HandleDeleteTgChat(w http.ResponseWriter, r *http.Request) {
	chatId, err := handler.getChatIdFromPath(r)
	if err != nil {
		handler.logger.Warn(
			"failed to parse chat id from query string",
			slog.String("error", err.Error()),
			slog.String("context", "handler.HandleDeleteTgChat"),
		)
		handler.writeError(w, http.StatusBadRequest,
			"Error while parsing chat id as integer",
			"bad_request",
			"bad_query_request_exception",
			"Could not parse chat id as integer",
			err,
		)
		return
	}
	err = handler.service.DeleteChat(chatId)
	if err != nil {
		if errors.Is(err, domain.ErrChatNotRegistered) {
			handler.writeError(w, http.StatusNotFound,
				"Chat not registered",
				"not_found",
				"bad_request_exception",
				fmt.Sprintf("Chat with id %d not registered", chatId),
				err,
			)
			return
		}
		handler.logger.Error(
			"failed to delete chat",
			slog.Int64("chat_id", chatId),
			slog.String("error", err.Error()),
			slog.String("context", "handler.HandleDeleteTgChat"),
		)
		handler.writeError(w, http.StatusInternalServerError,
			"Error while deleting chat",
			"internal_server_error",
			"delete_tgchat_exception",
			fmt.Sprintf("Could not delete chat with id: %d", chatId),
			err,
		)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (handler *Handler) HandleGetLinks(w http.ResponseWriter, r *http.Request) {
	chatId, err := handler.getChatIDFromHeader(r)
	if err != nil {
		handler.logger.Warn(
			"failed to parse chat id from header",
			slog.String("error", err.Error()),
			slog.String("context", "handler.HandleGetLinks"),
		)
		handler.writeError(w, http.StatusBadRequest,
			"Error while parsing chat id as integer",
			"bad_request",
			"bad_query_request_exception",
			"Could not parse chat id as integer",
			err,
		)
		return
	}
	links, err := handler.service.GetTrackedLinks(chatId)
	if err != nil {
		if errors.Is(err, domain.ErrChatNotRegistered) {
			handler.writeError(w, http.StatusNotFound,
				"Chat not registered",
				"not_found",
				"bad_request_exception",
				fmt.Sprintf("Chat with id %d not registered", chatId),
				err,
			)
			return
		}
		handler.logger.Error(
			"failed to get tracked links",
			slog.Int64("chat_id", chatId),
			slog.String("error", err.Error()),
			slog.String("context", "handler.HandleGetLinks"),
		)
		handler.writeError(w, http.StatusInternalServerError,
			"Error while getting links",
			"internal_server_error",
			"get_links_exception",
			fmt.Sprintf("Could not get links, tracked in chat wuth id %d", chatId),
			err,
		)
		return
	}

	type response struct {
		Links []linkResponse `json:"links"`
		Size  int32          `json:"size"`
	}

	var resp response
	resp.Size = int32(len(links))
	resp.Links = make([]linkResponse, len(links))

	for i, link := range links {
		resp.Links[i] = linkResponse{
			Id:      link.ID,
			Url:     link.URL,
			Tags:    link.Tags,
			Filters: link.Filters,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (handler *Handler) HandlePostLinks(w http.ResponseWriter, r *http.Request) {
	chatId, err := handler.getChatIDFromHeader(r)
	if err != nil {
		handler.logger.Warn(
			"failed to parse chat id from header",
			slog.String("error", err.Error()),
			slog.String("context", "handler.HandlePostLinks"),
		)
		handler.writeError(w, http.StatusBadRequest,
			"Error while parsing chat id as integer",
			"bad_request",
			"bad_query_request_exception",
			"Could not parse chat id as integer",
			err,
		)
		return
	}

	type addLinkRequest struct {
		Link    string   `json:"link"`
		Tags    []string `json:"tags"`
		Filters []string `json:"filters"`
	}

	var req addLinkRequest
	body, err := io.ReadAll(r.Body)
	if err != nil {
		handler.logger.Error(
			"failed to read request body",
			slog.String("error", err.Error()),
			slog.String("context", "handler.HandlePostLinks"),
		)
		handler.writeError(w, http.StatusInternalServerError,
			"Error while reading request body",
			"internal_server_error",
			"read_request_body_exception",
			"Error reading request body",
			err,
		)
		return
	}
	err = json.Unmarshal(body, &req)
	if err != nil {
		handler.writeError(w, http.StatusBadRequest,
			"Error unmarshalling request body",
			"bad_request",
			"bad_request_body_exception",
			"Could not unmarshal request body",
			err,
		)
		return
	}

	link, err := handler.service.AddLink(chatId, req.Link, req.Tags, req.Filters)
	if err != nil {
		if errors.Is(err, domain.ErrChatNotRegistered) {
			handler.writeError(w, http.StatusNotFound,
				"Chat not registered",
				"not_found",
				"chat_not_registered_exception",
				fmt.Sprintf("Chat %d not registered yet", chatId),
				err,
			)
			return
		}
		if errors.Is(err, domain.ErrAlreadySubscribed) {
			handler.writeError(w, http.StatusConflict,
				"Link already tracked",
				"conflict",
				"link_conflict_exception",
				fmt.Sprintf("Link %s already tracked", req.Link),
				err,
			)
			return
		}
		handler.logger.Error(
			"failed to add link",
			slog.Int64("chat_id", chatId),
			slog.String("error", err.Error()),
			slog.String("context", "handler.HandlePostLinks"),
			slog.String("link", req.Link),
		)
		handler.writeError(w, http.StatusInternalServerError,
			"Error while adding link",
			"internal_server_error",
			"add_link_exception",
			fmt.Sprintf("Could not add link %s to chat %d", req.Link, chatId),
			err,
		)
		return
	}

	resp := linkResponse{
		Id:      link.ID,
		Url:     link.URL,
		Tags:    link.Tags,
		Filters: link.Filters,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (handler *Handler) HandleDeleteLinks(w http.ResponseWriter, r *http.Request) {
	chatId, err := handler.getChatIDFromHeader(r)
	if err != nil {
		handler.logger.Warn(
			"failed to parse chat id from header",
			slog.String("error", err.Error()),
			slog.String("context", "handler.HandlePostLinks"),
		)
		handler.writeError(w, http.StatusBadRequest,
			"Error while parsing chat id as integer",
			"bad_request",
			"bad_query_request_exception",
			"Could not parse chat id as integer",
			err,
		)
		return
	}

	type request struct {
		Link string `json:"link"`
	}

	var req request

	body, err := io.ReadAll(r.Body)
	if err != nil {
		handler.logger.Error(
			"failed to read request body",
			slog.String("error", err.Error()),
			slog.String("context", "handler.HandleDeleteLinks"),
			slog.Int64("chat_id", chatId),
		)
		handler.writeError(w, http.StatusInternalServerError,
			"Error while reading request body",
			"internal_server_error",
			"read_request_body_exception",
			"Error reading request body",
			err,
		)
		return
	}
	err = json.Unmarshal(body, &req)
	if err != nil {
		handler.writeError(w, http.StatusBadRequest,
			"Error unmarshalling request body",
			"bad_request",
			"bad_request_body_exception",
			"Could not unmarshal request body",
			err,
		)
		return
	}
	link, err := handler.service.RemoveLink(chatId, req.Link)
	if err != nil {
		if errors.Is(err, domain.ErrChatNotRegistered) {
			handler.writeError(w, http.StatusNotFound,
				"Chat not registered",
				"not_found",
				"chat_not_registered_exception",
				fmt.Sprintf("Chat %d not registered yet", chatId),
				err,
			)
			return
		}
		if errors.Is(err, domain.ErrNotSubscribed) {
			handler.writeError(w, http.StatusNotFound,
				"Link not tracked",
				"not_found",
				"link_not_tracked_exception",
				fmt.Sprintf("Link %s not tracked", req.Link),
				err,
			)
			return
		}
		handler.logger.Error(
			"failed to delete link",
			slog.Int64("chat_id", chatId),
			slog.String("error", err.Error()),
			slog.String("context", "handler.HandleDeleteLinks"),
			slog.String("link", req.Link),
		)
		handler.writeError(w, http.StatusInternalServerError,
			"Error while deleting link",
			"internal_server_error",
			"delete_link_exception",
			fmt.Sprintf("Could not delete link %s in chat %d", req.Link, chatId),
			err,
		)
		return
	}

	resp := linkResponse{
		Id:      link.ID,
		Url:     link.URL,
		Tags:    link.Tags,
		Filters: link.Filters,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (handler *Handler) writeError(w http.ResponseWriter, status int, desc, code, excName, excMessage string, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := apiErrorResponse{
		Description:      desc,
		Code:             code,
		ExceptionName:    excName,
		ExceptionMessage: excMessage,
		Stacktrace:       []string{err.Error()},
	}
	_ = json.NewEncoder(w).Encode(resp)
}
