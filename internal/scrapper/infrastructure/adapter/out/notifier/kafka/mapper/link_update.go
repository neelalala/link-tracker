package mapper

import "encoding/json"

type linkUpdate struct {
	ID          int64   `json:"id"`
	URL         string  `json:"url"`
	Description string  `json:"description"`
	Preview     string  `json:"preview"`
	TgChatIDs   []int64 `json:"tgChatIds"`
}

func LinkUpdate(payload []byte) (map[string]any, error) {
	var update linkUpdate
	if err := json.Unmarshal(payload, &update); err != nil {
		return nil, err
	}

	tgChatIDs := make([]any, len(update.TgChatIDs))
	for i, id := range update.TgChatIDs {
		tgChatIDs[i] = id
	}

	return map[string]any{
		"id":          update.ID,
		"url":         update.URL,
		"description": update.Description,
		"preview":     update.Preview,
		"tgChatIds":   tgChatIDs,
	}, nil
}
