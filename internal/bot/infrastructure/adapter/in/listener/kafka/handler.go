package kafka

import (
	"encoding/json"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
	"log/slog"

	"github.com/IBM/sarama"
)

type Handler struct {
	updateHandler domain.LinkUpdateHandler
	log           *slog.Logger
}

func NewHandler(updateHandler domain.LinkUpdateHandler, log *slog.Logger) *Handler {
	return &Handler{
		updateHandler: updateHandler,
		log:           log,
	}
}

func (h *Handler) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *Handler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *Handler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case message, ok := <-claim.Messages():
			if !ok {
				return nil
			}

			h.log.Info("message received",
				slog.String("key", string(message.Key)),
				slog.String("value", string(message.Value)),
				slog.String("topic", message.Topic),
				slog.Int("partition", int(message.Partition)),
				slog.Int64("offset", message.Offset),
			)

			var updateJSON = struct {
				ID          int64   `json:"id"`
				URL         string  `json:"url"`
				Description string  `json:"description"`
				Preview     string  `json:"preview"`
				TgChatIds   []int64 `json:"tgChatIds"`
			}{}

			if err := json.Unmarshal(message.Value, &updateJSON); err != nil {
				h.log.Error("failed to unmarshal update", slog.String("error", err.Error()))
			}

			update := domain.LinkUpdate{
				ID:          updateJSON.ID,
				URL:         updateJSON.URL,
				Description: updateJSON.Description,
				Preview:     updateJSON.Preview,
				TgChatIDs:   updateJSON.TgChatIds,
			}

			if err := h.updateHandler.HandleUpdate(session.Context(), update); err != nil {
				h.log.Error("failed to handle update", slog.String("error", err.Error()))
				// retry logic?
			}

			session.MarkMessage(message, "")
		case <-session.Context().Done():
			return nil
		}

	}
}
