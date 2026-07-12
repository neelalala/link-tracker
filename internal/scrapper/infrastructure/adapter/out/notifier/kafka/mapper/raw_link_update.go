package mapper

import "encoding/json"

type rawLinkUpdate struct {
	ID          int64   `json:"id"`
	URL         string  `json:"url"`
	Author      string  `json:"author"`
	Description string  `json:"description"`
	TgChatIDs   []int64 `json:"tgChatIds"`
}

func RawLinkUpdateToNative(payload []byte) (map[string]any, error) {
	var update rawLinkUpdate
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
		"author":      update.Author,
		"description": update.Description,
		"tgChatIds":   tgChatIDs,
	}, nil
}
