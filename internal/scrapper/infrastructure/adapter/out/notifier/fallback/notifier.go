package fallback

import (
	"context"
	"errors"
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
	err1 := notifier.primary.SendUpdate(ctx, update)
	if err1 != nil {
		notifier.log.Warn("Primary notifier failed, trying fallback", "error", err1)
		err2 := notifier.secondary.SendUpdate(ctx, update)
		if err2 != nil {
			return errors.Join(err1, err2)
		}
		return nil
	}

	return nil
}
