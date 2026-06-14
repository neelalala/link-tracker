package domain

import (
	"context"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/validation"
)

type Priority string

const (
	PriorityHigh   Priority = "HIGH"
	PriorityMedium Priority = "MEDIUM"
	PriorityLow    Priority = "LOW"
)

type TrackedLink struct {
	ID   int64
	URL  string
	Tags []string
}

type LinkUpdate struct {
	URL         string
	Description string
	Priority    Priority
	TgChatIDs   []int64
}

type LinkUpdateHandler interface {
	HandleUpdate(ctx context.Context, update LinkUpdate) error
}

func (update LinkUpdate) Validate() validation.Problems {
	problems := make(validation.Problems)

	if update.URL == "" {
		problems.Add("url", "must not be empty")
	}

	if update.Description == "" {
		problems.Add("description", "must not be empty")
	}

	switch update.Priority {
	case PriorityHigh:
	case PriorityMedium:
	case PriorityLow:
	default:
		problems.Add("priority", "must be one of: HIGH, MEDIUM, LOW")
	}

	if len(update.TgChatIDs) == 0 {
		problems.Add("tgChatIDs", "must contain least one chat id")
	}

	return problems
}
