package mapper

import "encoding/json"

type processedLinkUpdate struct {
	URL         string  `json:"url"`
	Author      string  `json:"author"`
	Description string  `json:"description"`
	Priority    string  `json:"priority"`
	TgChatIDs   []int64 `json:"tgChatIds"`
}

func ProcessedLinkUpdateToNative(payload []byte) (map[string]any, error) {
	var update processedLinkUpdate
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
		"priority":    update.Priority,
		"tgChatIds":   tgChatIDs,
	}, nil
}
