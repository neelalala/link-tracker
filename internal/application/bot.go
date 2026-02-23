package application

import (
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/telegram"
	"log"
	"strings"
)

type Bot struct {
	api    *telegram.BotApi
	router *Router
}

func NewBot(token string, router *Router) (*Bot, error) {
	api, err := telegram.NewBot(token)
	if err != nil {
		return nil, err
	}
	return &Bot{
		api:    api,
		router: router,
	}, nil
}
func (b *Bot) Start() {
	for {
		updates, err := b.api.GetUpdates()
		if err != nil {
			log.Println(err)
		}
		for _, update := range updates {
			err := b.handleMessage(update)
			if err != nil {
				log.Println(err)
			}
		}
	}
}

func (b *Bot) handleMessage(msg domain.Message) error {
	if strings.HasPrefix(msg.Text, "/") {
		ss := strings.Split(msg.Text, " ")
		resp := b.router.Handle(ss[0], msg.From)
		return b.api.SendMessage(msg.ChatID, resp)
	}
	return nil
}
