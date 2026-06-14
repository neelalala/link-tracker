package sender

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/domain"
)

type Outbox struct {
	outRepo domain.OutboxRepository
	topic   string

	log *slog.Logger
}

func NewOutbox(outRepo domain.OutboxRepository, topic string, log *slog.Logger) Outbox {
	return Outbox{
		outRepo: outRepo,
		topic:   topic,
		log:     log,
	}
}

func (sender Outbox) SendUpdate(ctx context.Context, update domain.ProcessedLinkUpdate) error {
	sender.log.Debug("Sending update",
		"url", update.URL,
		"tgChats", len(update.TgChatIDs),
	)

	var updateJSON = struct {
		URL         string  `json:"url"`
		Description string  `json:"description"`
		Priority    string  `json:"priority"`
		TgChatIDs   []int64 `json:"tgChatIds"`
	}{
		URL:         update.URL,
		Description: update.Description,
		Priority:    string(update.Priority),
		TgChatIDs:   update.TgChatIDs,
	}

	payload, err := json.Marshal(&updateJSON)
	if err != nil {
		return fmt.Errorf("error marshalling update: %w", err)
	}

	_, err = sender.outRepo.Add(ctx, sender.topic, payload)
	if err != nil {
		return fmt.Errorf("error sending update: %w", err)
	}

	return nil
}
