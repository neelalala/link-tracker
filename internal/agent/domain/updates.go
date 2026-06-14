package domain

import "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/validation"

type Priority string

const (
	PriorityHigh   Priority = "HIGH"
	PriorityMedium Priority = "MEDIUM"
	PriorityLow    Priority = "LOW"
)

type LinkUpdate struct {
	ID          int64
	URL         string
	Author      string
	Description string
	Preview     string
	TgChatIDs   []int64
}

func (update LinkUpdate) Validate() validation.Problems {
	problems := make(validation.Problems)

	if update.ID < 0 {
		problems.Add("id", "must be positive")
	}

	if update.URL == "" {
		problems.Add("url", "must be not empty")
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

type ProcessedLinkUpdate struct {
	URL         string
	Author      string
	Description string
	Preview     string
	TgChatIDs   []int64
}
