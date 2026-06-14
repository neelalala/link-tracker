package application

import (
	"context"
	"fmt"
	"log/slog"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/domain"
)

type Service struct {
	filters     []domain.Filter
	transformer domain.Transformer

	sender domain.UpdateSender

	log *slog.Logger
}

func NewService(
	filters []domain.Filter,
	transformer domain.Transformer,
	sender domain.UpdateSender,
	log *slog.Logger,
) *Service {
	return &Service{
		filters:     filters,
		transformer: transformer,
		log:         log,
	}
}

func (service *Service) HandleUpdate(ctx context.Context, update domain.LinkUpdate) error {
	service.log.Debug("Handling update",
		"url", update.URL,
	)

	service.log.Debug("Filtering update",
		"url", update.URL,
	)

	for _, filter := range service.filters {
		if !filter.Check(update) {
			service.log.Debug("Filter skipped",
				"url", update.URL,
			)
			return nil
		}
	}
	service.log.Debug("Filter succeeded",
		"url", update.URL,
	)

	service.log.Debug("Transforming update",
		"url", update.URL,
	)

	transformed, err := service.transformer.Transform(ctx, update)
	if err != nil {
		return fmt.Errorf("error transforming update: %w", err)
	}
	service.log.Debug("Transform succeeded",
		"url", update.URL,
	)

	if err := service.sender.SendUpdate(ctx, transformed); err != nil {
		return fmt.Errorf("error sending update: %w", err)
	}

	service.log.Debug("Update handled successfully",
		"url", update.URL,
	)

	return nil
}
