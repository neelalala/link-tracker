package domain

type Command struct {
	Name        string
	Description string
	Do          func(user User) string
}

func HandleStart(user User) string {
	return "Hi! This bot can track updates on your links, so you won't miss on news! /help for list my commands"
}

func HandleHelp(user User) string {
	return `Available commands:
/start - what this bot can do
/help - list all available commands`
}
