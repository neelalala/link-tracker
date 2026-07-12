package filters

import (
	"strings"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/domain"
)

type Words map[string]struct{}

func NewWords(excluded []string) Words {
	filter := make(Words, len(excluded))
	for _, word := range excluded {
		filter[word] = struct{}{}
	}
	return filter
}

func (filter Words) Check(update domain.LinkUpdate) bool {
	words := strings.Fields(update.Description)
	for _, word := range words {
		if _, ok := filter[word]; ok {
			return false
		}
	}
	return true
}
