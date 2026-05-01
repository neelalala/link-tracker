package commands

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type fakeCommand struct {
	name string
	desc string
}

func (f fakeCommand) Name() string {
	return f.name
}

func (f fakeCommand) Description() string {
	return f.desc
}

func (f fakeCommand) Execute(ctx context.Context, msg domain.Message) (string, error) {
	return "", nil
}

func TestHelpCommand_Execute(t *testing.T) {
	cmds := []domain.Command{
		fakeCommand{name: "start", desc: "Start the bot"},
		fakeCommand{name: "track", desc: "Track a link"},
	}

	cmd := NewHelpCommand()
	cmd.SetCommands(cmds)

	sb := strings.Builder{}
	sb.WriteString(helpMessageHeader)
	for _, cmd := range cmds {
		sb.WriteString(fmt.Sprintf("/%s – %s\n", cmd.Name(), cmd.Description()))
	}
	expectedResult := sb.String()

	res, err := cmd.Execute(context.Background(), domain.Message{ChatID: 123})

	assert.NoError(t, err)
	assert.Equal(t, expectedResult, res)
}
