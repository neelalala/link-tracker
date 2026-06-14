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
	summarizer  domain.Summarizer

	log *slog.Logger
}

func NewService(
	filters []domain.Filter,
	transformer domain.Transformer,
	summarizer domain.Summarizer,
	log *slog.Logger,
) *Service {
	return &Service{
		filters:     filters,
		transformer: transformer,
		summarizer:  summarizer,
		log:         log,
	}
}

func (service *Service) Filter(_ context.Context, update domain.LinkUpdate) bool {
	service.log.Debug("Filtering update",
		"url", update.URL,
	)

	for _, filter := range service.filters {
		if !filter.Check(update) {
			return false
		}
	}

	return true
}

func (service *Service) Transform(ctx context.Context, update domain.LinkUpdate) (domain.ProcessedLinkUpdate, error) {
	service.log.Debug("Transforming update",
		"url", update.URL,
	)

	transformed, err := service.transformer.Transform(ctx, update)
	if err != nil {
		return domain.ProcessedLinkUpdate{}, fmt.Errorf("error transforming update: %w", err)
	}

	return transformed, nil
}
