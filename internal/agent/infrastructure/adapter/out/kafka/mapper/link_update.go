package mapper

import "encoding/json"

type linkUpdate struct {
	URL         string  `json:"url"`
	Author      string  `json:"author"`
	Description string  `json:"description"`
	TgChatIDs   []int64 `json:"tgChatIds"`
}

func LinkUpdateToNative(payload []byte) (map[string]any, error) {
	var update linkUpdate
	if err := json.Unmarshal(payload, &update); err != nil {
		return nil, err
	}

	tgChatIDs := make([]any, len(update.TgChatIDs))
	for i, id := range update.TgChatIDs {
		tgChatIDs[i] = id
	}

	return map[string]any{
		"url":         update.URL,
		"author":      update.Author,
		"description": update.Description,
		"tgChatIds":   tgChatIDs,
	}, nil
}
