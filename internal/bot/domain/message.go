package domain

import "strings"

type Message struct {
	ID     int64
	ChatID int64
	Text   string
}

func (m *Message) IsCommand() bool {
	return strings.HasPrefix(m.Text, "/") && len(m.Text) > 1
}

func (m *Message) ParseCommand() (string, []string) {
	if !m.IsCommand() {
		return "", nil
	}

	args := strings.Fields(m.Text)
	if len(args) == 0 {
		return "", nil
	}

	cmd := strings.TrimPrefix(args[0], "/")

	if len(args) == 1 {
		return cmd, nil
	}

	return cmd, args[1:]
}
