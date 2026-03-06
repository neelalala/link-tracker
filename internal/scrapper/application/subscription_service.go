package application

import (
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type SubscriptionService interface {
	RegisterChat(chatID int64) error
	DeleteChat(chatID int64) error

	GetTrackedLinks(chatID int64) ([]domain.TrackedLink, error)
	AddLink(chatID int64, url string, tags, filters []string) (domain.TrackedLink, error)
	RemoveLink(chatID int64, url string) (domain.TrackedLink, error)
}
