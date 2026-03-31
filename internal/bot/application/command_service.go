package application

import (
	"context"
	"errors"
	"fmt"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

var ErrNotACommand = errors.New("message is not a command")

type CommandService struct {
	scrapper    domain.Scrapper
	sessionRepo domain.SessionRepository
	commands    map[string]domain.Command
}

func NewCommandService(
	scrapper domain.Scrapper,
	sessionRepo domain.SessionRepository,
	commands []domain.Command,
) *CommandService {
	service := &CommandService{
		scrapper:    scrapper,
		sessionRepo: sessionRepo,
		commands:    make(map[string]domain.Command),
	}

	for _, command := range commands {
		service.commands[command.Name()] = command
	}

	return service
}

func (service *CommandService) HandleCommand(ctx context.Context, msg domain.Message) (string, error) {
	command, _ := msg.ParseCommand()
	if command == "" {
		return "", ErrNotACommand
	}

	fmt.Println(msg, command)
	cmd, ok := service.commands[command]
	if !ok {
		return "Unknown command. Use /help to list all commands", nil
	}

	resp, err := cmd.Execute(ctx, msg)
	if err != nil {
		return "", err
	}

	return resp, nil
}

func (service *CommandService) GetCommandsInfo() []domain.CommandInfo {
	var infos []domain.CommandInfo

	for _, command := range service.commands {
		infos = append(infos, domain.CommandInfo{
			Name:        command.Name(),
			Description: command.Description(),
		})
	}

	return infos
}
