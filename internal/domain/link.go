package domain

type Link struct {
	ChatID int64
	UserID int64
	URL    string
	Tags   []string
}

type LinkUpdateHandler interface {
	HandleUpdate(update LinkUpdate) error
}
type LinkUpdate struct {
	ID          int64
	URL         string
	Description string
	TgChatIDs   []int64
}
