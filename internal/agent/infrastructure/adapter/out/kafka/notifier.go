package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/domain"
)

type Notifier struct {
	outRepo domain.OutboxRepository
	topic   string

	log *slog.Logger
}

func NewNotifier(outRepo domain.OutboxRepository, topic string, log *slog.Logger) *Notifier {
	return &Notifier{
		outRepo: outRepo,
		topic:   topic,
		log:     log,
	}
}

type processedUpdate struct {
	URL         string  `json:"url"`
	Author      string  `json:"author"`
	Description string  `json:"description"`
	Preview     string  `json:"preview"`
	TgChatIDs   []int64 `json:"tgChatIds"`
}

func (notifier *Notifier) SendUpdate(ctx context.Context, update domain.LinkUpdate) error {
	notifier.log.Info("sending update",
		"topic", notifier.topic,
		"url", update.URL,
		"description", update.Description,
		"tgChatIDs", update.TgChatIDs,
	)

	updateJSON := processedUpdate{
		URL:         update.URL,
		Author:      update.Author,
		Description: update.Description,
		Preview:     update.Preview,
		TgChatIDs:   update.TgChatIDs,
	}

	payload, err := json.Marshal(updateJSON)
	if err != nil {
		return fmt.Errorf("failed to encode update: %w", err)
	}

	_, err = notifier.outRepo.Add(ctx, notifier.topic, payload)
	if err != nil {
		return fmt.Errorf("failed to send update: %w", err)
	}

	return nil
}
