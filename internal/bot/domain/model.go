package domain

type Command struct {
	Name        string
	Description string
}

type Message struct {
	ID     int64
	ChatID int64
	Text   string
}

type TrackedLink struct {
	ID   int64
	URL  string
	Tags []string
}

type LinkUpdate struct {
	ID          int64
	URL         string
	Description string
	TgChatIDs   []int64
}
