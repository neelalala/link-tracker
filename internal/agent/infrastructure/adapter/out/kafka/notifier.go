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

type linkUpdate struct {
	URL         string  `json:"url"`
	Author      string  `json:"author"`
	Description string  `json:"description"`
	Priority    string  `json:"priority"`
	TgChatIDs   []int64 `json:"tgChatIds"`
}

func (notifier *Notifier) SendUpdate(ctx context.Context, update domain.ProcessedLinkUpdate) error {
	notifier.log.Info("sending update",
		"topic", notifier.topic,
		"url", update.URL,
		"description", update.Description,
		"tgChatIDs", update.TgChatIDs,
	)

	updateJSON := linkUpdate{
		URL:         update.URL,
		Author:      update.Author,
		Description: update.Description,
		Priority:    string(update.Priority),
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
