package filters

import "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/domain"

type Length int

func NewLength(minLength int) Length {
	return Length(minLength)
}

func (filter Length) Check(update domain.LinkUpdate) bool {
	if len(update.Description) > int(filter) {
		return true
	}
	return false
}
