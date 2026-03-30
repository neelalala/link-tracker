package commands

import (
	"context"
	"fmt"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
	"strings"
)

const (
	helpCommandName        = "help"
	helpCommandDescription = "List all available commands"

	helpMessageHeader = "Available commands:\n"
)

type HelpCommand struct {
	commands []domain.Command
}

func NewHelpCommand() *HelpCommand {
	return &HelpCommand{}
}

func (c *HelpCommand) SetCommands(commands []domain.Command) {
	c.commands = commands
}

func (c *HelpCommand) Name() string {
	return helpCommandName
}

func (c *HelpCommand) Description() string {
	return helpCommandDescription
}

func (c *HelpCommand) Execute(ctx context.Context, msg domain.Message) (string, error) {
	sb := strings.Builder{}
	sb.WriteString(helpMessageHeader)

	for _, command := range c.commands {
		sb.WriteString(fmt.Sprintf("/%s – %s\n", command.Name(), command.Description()))
	}

	return sb.String(), nil
}
