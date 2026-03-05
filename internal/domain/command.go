package domain

type CommandHandler func(user User, chatID int64, args []string) string

type Command struct {
	Name        string
	Description string
	Do          CommandHandler
}
