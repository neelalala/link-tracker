package domain

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
