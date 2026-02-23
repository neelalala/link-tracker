package domain

type Command struct {
	Name        string
	Description string
	Do          func(user User) string
}
