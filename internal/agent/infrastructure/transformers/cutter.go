package transformers

import (
	"context"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/domain"
)

type Cutter int

func NewCutter(threshold int) Cutter {
	return Cutter(threshold)
}

func (transformer Cutter) Transform(_ context.Context, raw domain.LinkUpdate) (domain.ProcessedLinkUpdate, error) {
	return domain.ProcessedLinkUpdate{
		URL:         raw.URL,
		Description: truncateText(raw.Description, int(transformer)),
		Priority:    domain.PriorityHigh,
		TgChatIDs:   raw.TgChatIDs,
	}, nil
}

func truncateText(s string, maxLen int) string {
	runes := []rune(s)

	if len(runes) <= maxLen {
		return s
	}

	return string(runes[:maxLen-3]) + "..."
}
