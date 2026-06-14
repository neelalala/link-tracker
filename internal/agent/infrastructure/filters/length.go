package filters

import "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/domain"

type Length int

func NewLength(length int) Length {
	return Length(length)
}

func (filter Length) Check(update domain.LinkUpdate) bool {
	if len(update.Description) > int(filter) {
		return false
	}
	return true
}
