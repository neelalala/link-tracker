package application

import (
	"context"
	"errors"
	"github.com/stretchr/testify/assert"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
	"testing"
)

const testCommandExecuted = "Command executed"

var testCommandError = errors.New("command error")

type fakeCommand struct {
	name string
	desc string
	res  string
	err  error
}

func (f fakeCommand) Name() string {
	return f.name
}

func (f fakeCommand) Description() string {
	return f.desc
}

func (f fakeCommand) Execute(ctx context.Context, msg domain.Message) (string, error) {
	return f.res, f.err
}

func TestCommandService_HandleCommand(t *testing.T) {

	cmdSuccess := fakeCommand{
		name: "test",
		desc: "test command",
		res:  testCommandExecuted,
		err:  nil,
	}
	cmdError := fakeCommand{
		name: "fail",
		desc: "failing command",
		res:  "",
		err:  testCommandError,
	}

	service := NewCommandService(nil, nil, []domain.Command{cmdSuccess, cmdError})

	tests := []struct {
		name           string
		msg            domain.Message
		expectedResult string
		expectedError  error
	}{
		{
			name:           "Not a command",
			msg:            domain.Message{Text: "just text"},
			expectedResult: "",
			expectedError:  ErrNotACommand,
		},
		{
			name:           "Unknown command",
			msg:            domain.Message{Text: "/unknown"},
			expectedResult: "Unknown command. Use /help to list all commands",
			expectedError:  nil,
		},
		{
			name:           "Success execution",
			msg:            domain.Message{Text: "/test"},
			expectedResult: testCommandExecuted,
			expectedError:  nil,
		},
		{
			name:           "Command returns error",
			msg:            domain.Message{Text: "/fail"},
			expectedResult: "",
			expectedError:  testCommandError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := service.HandleCommand(context.Background(), tt.msg)

			assert.Equal(t, tt.expectedResult, res)
			if tt.expectedError != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCommandService_GetCommandsInfo(t *testing.T) {
	cmd1 := fakeCommand{name: "cmd1", desc: "first"}
	cmd2 := fakeCommand{name: "cmd2", desc: "second"}

	service := NewCommandService(nil, nil, []domain.Command{cmd1, cmd2})

	infos := service.GetCommandsInfo()

	assert.Len(t, infos, 2)
	names := []string{infos[0].Name, infos[1].Name}
	assert.Contains(t, names, "cmd1")
	assert.Contains(t, names, "cmd2")
}
