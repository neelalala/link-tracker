package kafka

import "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/validation"

type LinkUpdateJSON struct {
	ID          int64   `json:"id"`
	URL         string  `json:"url"`
	Description string  `json:"description"`
	Preview     string  `json:"preview"`
	TgChatIDs   []int64 `json:"tgChatIds"`
}

func (update LinkUpdateJSON) Validate() validation.Problems {
	problems := make(validation.Problems)

	if update.ID <= 0 {
		problems.Add("id", "must be positive")
	}

	if update.URL == "" {
		problems.Add("url", "must not be empty")
	}

	if update.Description == "" {
		problems.Add("description", "must not be empty")
	}

	if update.Preview == "" {
		problems.Add("preview", "must not be empty")
	}

	if len(update.TgChatIDs) == 0 {
		problems.Add("tgChatIds", "must contain least one chat id")
	}

	return problems
}
