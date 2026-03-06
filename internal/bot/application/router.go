package application

import (
	domain2 "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type Router struct {
	commands map[string]domain2.Command
}

func NewRouter(cmds []domain2.Command) *Router {
	r := &Router{
		commands: make(map[string]domain2.Command),
	}
	for _, cmd := range cmds {
		r.commands["/"+cmd.Name] = cmd
	}
	return r
}

func (router *Router) Handle(command string, args []string, user domain2.User, chatID int64) string {
	cmd, exists := router.commands[command]
	if !exists {
		return "Unknown command. Use /help to list all commands."
	}
	return cmd.Do(user, chatID, args)
}
