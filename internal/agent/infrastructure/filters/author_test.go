package filters

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/domain"
)

func makeUpdate(author, description string) domain.LinkUpdate {
	return domain.LinkUpdate{
		URL:         "https://example.com",
		Author:      author,
		Description: description,
		TgChatIDs:   []int64{1},
	}
}

func TestAuthor_Check_ExcludedAuthor(t *testing.T) {
	filter := NewAuthor([]string{"bot", "spammer"})
	update1 := makeUpdate("bot", "some update")
	update2 := makeUpdate("spammer", "another update")

	assert.False(t, filter.Check(update1))
	assert.False(t, filter.Check(update2))
}

func TestAuthor_Check_AllowedAuthor(t *testing.T) {
	filter := NewAuthor([]string{"bot"})
	update := makeUpdate("alice", "legitimate update")

	assert.True(t, filter.Check(update))
}

func TestAuthor_Check_EmptyExcludedList(t *testing.T) {
	filter := NewAuthor([]string{})
	update := makeUpdate("anyone", "update")

	assert.True(t, filter.Check(update))
}
