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

	id, ok := record["id"].(int64)
	if !ok {
		return domain.LinkUpdate{}, fmt.Errorf("failed to cast native to int64 id")
	}

	url, ok := record["url"].(string)
	if !ok {
		return domain.LinkUpdate{}, fmt.Errorf("failed to cast native to string url")
	}

	description, ok := record["description"].(string)
	if !ok {
		return domain.LinkUpdate{}, fmt.Errorf("failed to cast native to string description")
	}

	preview, ok := record["preview"].(string)
	if !ok {
		return domain.LinkUpdate{}, fmt.Errorf("failed to cast native to string preview")
	}

	update := domain.LinkUpdate{
		ID:          id,
		URL:         url,
		Description: description,
		Preview:     preview,
	}

	ids, ok := record["tgChatIds"].([]int64)
	if !ok {
		return domain.LinkUpdate{}, fmt.Errorf("failed to cast native to int64 ids")
	}

	update.TgChatIDs = make([]int64, len(ids))
	for i, val := range ids {
		update.TgChatIDs[i] = val
	}

	return update, nil
}
