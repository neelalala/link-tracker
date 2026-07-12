package mapper

import (
	"fmt"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

func ProcessedLinkUpdateFromNative(native any) (domain.LinkUpdate, error) {
	record, ok := native.(map[string]any)
	if !ok {
		return domain.LinkUpdate{}, fmt.Errorf("failed to cast native to map[string]any")

	}

	url, ok := record["url"].(string)
	if !ok {
		return domain.LinkUpdate{}, fmt.Errorf("failed to cast native to string url")
	}

	description, ok := record["description"].(string)
	if !ok {
		return domain.LinkUpdate{}, fmt.Errorf("failed to cast native to string description")
	}

	priority, ok := record["priority"].(string)
	if !ok {
		return domain.LinkUpdate{}, fmt.Errorf("failed to cast native to string priority")
	}

	update := domain.LinkUpdate{
		URL:         url,
		Description: description,
		Priority:    domain.Priority(priority),
	}

	ids, ok := record["tgChatIds"].([]any)
	if !ok {
		return domain.LinkUpdate{}, fmt.Errorf("failed to cast tgChatIds to []any")
	}

	update.TgChatIDs = make([]int64, len(ids))
	for i, val := range ids {
		id, ok := val.(int64)
		if !ok {
			return domain.LinkUpdate{}, fmt.Errorf("failed to cast tgChatIds[%d] to int64", i)
		}
		update.TgChatIDs[i] = id
	}

	return update, nil
}
