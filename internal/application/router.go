package application

import (
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

type Router struct {
	commands map[string]domain.Command
}

func NewRouter(cmds []domain.Command) *Router {
	r := &Router{
		commands: make(map[string]domain.Command),
	}
	for _, cmd := range cmds {
		r.commands["/"+cmd.Name] = cmd
	}
	return r
}

func (router *Router) Handle(command string, args []string, user domain.User, chatID int64) string {
	cmd, exists := router.commands[command]
	if !exists {
		return "Unknown command. Use /help to list all commands."
	}
	return cmd.Do(user, chatID, args)
}
