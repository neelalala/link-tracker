package mapper

import (
	"fmt"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

func LinkUpdateFromNative(native any) (domain.LinkUpdate, error) {
	record, ok := native.(map[string]any)
	if !ok {
		return domain.LinkUpdate{}, fmt.Errorf("failed to cast native to map[string]any")

	}

	update := domain.LinkUpdate{
		ID:          record["id"].(int64),
		URL:         record["url"].(string),
		Description: record["description"].(string),
		Preview:     record["preview"].(string),
	}

	if rawIDs, ok := record["tgChatIds"].([]any); ok {
		update.TgChatIDs = make([]int64, len(rawIDs))
		for i, val := range rawIDs {
			update.TgChatIDs[i] = val.(int64)
		}
	}

	return update, nil
}
