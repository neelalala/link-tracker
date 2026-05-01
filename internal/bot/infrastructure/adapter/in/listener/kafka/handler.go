package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/IBM/sarama"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/validation"
)

type Handler struct {
	updateHandler domain.LinkUpdateHandler
	producer      sarama.SyncProducer
	dlqTopic      string
	retries       int

	log *slog.Logger
}

func NewHandler(
	updateHandler domain.LinkUpdateHandler,
	producer sarama.SyncProducer,
	dqlTopic string,
	retries int,
	log *slog.Logger) *Handler {
	return &Handler{
		updateHandler: updateHandler,
		producer:      producer,
		dlqTopic:      dqlTopic,
		retries:       retries,
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

			var updateJSON LinkUpdateJSON

			if err := json.Unmarshal(message.Value, &updateJSON); err != nil {
				h.log.Error("failed to unmarshal update", slog.String("error", err.Error()))
				h.sendToDLQ(message, fmt.Sprintf("unmarshal error: %v", err))
				session.MarkMessage(message, "")
				continue
			}

			if err := validation.Check(updateJSON); err != nil {
				h.log.Error("failed to validate update", slog.String("error", err.Error()))
				h.sendToDLQ(message, fmt.Sprintf("validate error: %v", err))
				session.MarkMessage(message, "")
				continue
			}

			update := domain.LinkUpdate{
				ID:          updateJSON.ID,
				URL:         updateJSON.URL,
				Description: updateJSON.Description,
				Preview:     updateJSON.Preview,
				TgChatIDs:   updateJSON.TgChatIDs,
			}

			err := h.handleUpdate(session.Context(), update)
			if err != nil {
				h.log.Error("failed to handle update", slog.String("error", err.Error()))
				h.sendToDLQ(message, fmt.Sprintf("handle update error: %v", err))
			}

			session.MarkMessage(message, "")
		case <-session.Context().Done():
			return nil
		}

	}
}

func (h *Handler) handleUpdate(ctx context.Context, update domain.LinkUpdate) error {
	var processErr error
	for i := range h.retries {
		if err := h.updateHandler.HandleUpdate(ctx, update); err != nil {
			processErr = err
			h.log.Warn("failed to handle update",
				slog.String("error", err.Error()),
				slog.Int("attempt", i+1),
				slog.Int64("link_id", update.ID))
			time.Sleep(time.Second)
			continue
		}
		return nil
	}
	return processErr
}

func (h *Handler) sendToDLQ(msg *sarama.ConsumerMessage, reason string) {
	dlqMsg := &sarama.ProducerMessage{
		Topic: h.dlqTopic,
		Key:   sarama.ByteEncoder(msg.Key),
		Value: sarama.ByteEncoder(msg.Value),
		Headers: []sarama.RecordHeader{
			{Key: []byte("reason"), Value: []byte(reason)},
			{Key: []byte("topic"), Value: []byte(msg.Topic)},
			{Key: []byte("partition"), Value: []byte(fmt.Sprintf("%d", msg.Partition))},
			{Key: []byte("offset"), Value: []byte(fmt.Sprintf("%d", msg.Offset))},
		},
	}

	partition, offset, err := h.producer.SendMessage(dlqMsg)
	if err != nil {
		h.log.Error("failed to send message to DLQ", slog.String("error", err.Error()))
		return
	}

	h.log.Info("message sent to DLQ successfully",
		slog.Int("dlq_partition", int(partition)),
		slog.Int64("dlq_offset", offset),
	)
}
