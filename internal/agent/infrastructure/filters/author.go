package filters

import (
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/domain"
)

type Author map[string]struct{}

func NewAuthor(excluded []string) Author {
	filter := make(Author, len(excluded))
	for _, author := range excluded {
		filter[author] = struct{}{}
	}
	return filter
}

func (filter Author) Check(update domain.LinkUpdate) bool {
	if _, ok := filter[update.Author]; ok {
		return false
	}
	return true
}
