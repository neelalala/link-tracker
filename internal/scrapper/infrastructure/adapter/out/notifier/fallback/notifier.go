package fallback

import (
	"context"
	"log/slog"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type UpdateNotifier struct {
	primary   domain.UpdateNotifier
	secondary domain.UpdateNotifier
	log       *slog.Logger
}

func New(primary, secondary domain.UpdateNotifier, log *slog.Logger) *UpdateNotifier {
	return &UpdateNotifier{
		primary:   primary,
		secondary: secondary,
		log:       log,
	}
}

func (notifier *UpdateNotifier) SendUpdate(ctx context.Context, update domain.LinkUpdate) error {
	err := notifier.primary.SendUpdate(ctx, update)
	if err != nil {
		notifier.log.Warn("Primary notifier failed, trying fallback", "error", err)
		return notifier.secondary.SendUpdate(ctx, update)
	}

	return nil
}
