package domain

import (
	"context"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/validation"
)

type TrackedLink struct {
	ID   int64
	URL  string
	Tags []string
}

type LinkUpdate struct {
	ID          int64
	URL         string
	Description string
	Preview     string
	TgChatIDs   []int64
}

type LinkUpdateHandler interface {
	HandleUpdate(ctx context.Context, update LinkUpdate) error
}

func (update LinkUpdate) Validate() validation.Problems {
	problems := make(validation.Problems)

	if update.ID < 0 {
		problems.Add("id", "must be positive")
	}

	if update.URL == "" {
		problems.Add("url", "must not be empty")
	}

	if update.Description == "" && update.Preview == "" {
		problems.Add("description", "either description or preview must be set")
		problems.Add("preview", "either description or preview must be set")
	}

	if len(update.TgChatIDs) == 0 {
		problems.Add("tgChatIDs", "must contain least one chat id")
	}

	return problems
}
