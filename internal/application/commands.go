package application

import "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"

func GetCommands() []domain.Command {
	return []domain.Command{
		{
			Name:        "start",
			Description: "What this bot can do",
			Do:          HandleStart,
		},
		{
			Name:        "help",
			Description: "List all available commands",
			Do:          HandleHelp,
		},
	}
}

func HandleStart(user domain.User, chatID int64, args []string) string {
	return "Hi! This bot can track updates on your links, so you won't miss on news! /help for list my commands"
}

func HandleHelp(user domain.User, chatID int64, args []string) string {
	return `Available commands:
/start - what this bot can do
/help - list all available commands`
}
