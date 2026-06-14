package transformers

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/domain"
)

func makeLinkUpdate(description string) domain.LinkUpdate {
	return domain.LinkUpdate{
		URL:         "https://example.com",
		Author:      "alice",
		Description: description,
		TgChatIDs:   []int64{1},
	}
}

func TestCutter_Transform_LongText(t *testing.T) {
	const threshold = 20
	cutter := NewCutter(threshold)

	longText := strings.Repeat("1234", 100)
	result, err := cutter.Transform(context.Background(), makeLinkUpdate(longText))
	require.NoError(t, err)

	assert.NotEqual(t, longText, result.Description)
	assert.Equal(t, len([]rune(result.Description)), threshold)
	assert.True(t, strings.HasSuffix(result.Description, "..."))
}

func TestCutter_Transform_ShortText(t *testing.T) {
	const threshold = 100
	cutter := NewCutter(threshold)

	shortText := "short update"
	result, err := cutter.Transform(context.Background(), makeLinkUpdate(shortText))
	require.NoError(t, err)

	assert.Equal(t, shortText, result.Description)
}

func TestCutter_Transform_TextExactlyAtThreshold(t *testing.T) {
	text := "hello"
	cutter := NewCutter(len(text))
	result, err := cutter.Transform(context.Background(), makeLinkUpdate(text))
	require.NoError(t, err)

	assert.Equal(t, text, result.Description)
}

func TestCutter_Transform_PreservesOtherFields(t *testing.T) {
	cutter := NewCutter(50)
	update := domain.LinkUpdate{
		URL:         "https://example.com/repo",
		Author:      "bob",
		Description: "a short description",
		TgChatIDs:   []int64{42, 99},
	}

	result, err := cutter.Transform(context.Background(), update)
	require.NoError(t, err)

	assert.Equal(t, update.URL, result.URL)
	assert.Equal(t, update.Author, result.Author)
	assert.Equal(t, domain.PriorityHigh, result.Priority)
	assert.Equal(t, update.TgChatIDs, result.TgChatIDs)

}
