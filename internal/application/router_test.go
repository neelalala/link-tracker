package application

import (
	"github.com/stretchr/testify/assert"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	"testing"
)

func TestRouter_Handle(t *testing.T) {
	cmds := GetCommands()
	router := NewRouter(cmds)

	testUser := domain.User{
		ID:       12345,
		Username: "test",
		Name:     "Test",
	}

	tests := []struct {
		name     string
		command  string
		expected string
	}{
		{
			name:     "Positive: /start command returns greeting",
			command:  "/start",
			expected: "Hi! This bot can track updates on your links, so you won't miss on news! /help for list my commands",
		},
		{
			name:     "Positive: /help command returns command list",
			command:  "/help",
			expected: "Available commands:\n/start - what this bot can do\n/help - list all available commands",
		},
		{
			name:     "Negative: unknown command returns error message",
			command:  "/cmd",
			expected: "Unknown command. Use /help to list all commands.",
		},
		{
			name:     "Negative: missing slash returns error message",
			command:  "start",
			expected: "Unknown command. Use /help to list all commands.",
		},
		{
			name:     "Negative: double slash returns error message",
			command:  "//start",
			expected: "Unknown command. Use /help to list all commands.",
		},
		{
			name:     "Negative: empty string",
			command:  "",
			expected: "Unknown command. Use /help to list all commands.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := router.Handle(tt.command, testUser)

			assert.Equal(t, tt.expected, actual, "\nExpected: %s\nActual: %s", tt.expected, actual)
		})
	}
}
