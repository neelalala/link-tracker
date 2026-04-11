package stackoverflow

import (
	"fmt"
	"time"
)

type StackoverflowAnswerUpdate struct {
	Title     string
	Owner     string
	CreatedAt time.Time
	Body      string

	MaxPreviewLen int
}

func (soAnswerUpdate *StackoverflowAnswerUpdate) UpdatedAt() time.Time {
	return soAnswerUpdate.CreatedAt
}

func (soAnswerUpdate *StackoverflowAnswerUpdate) Description() string {
	return fmt.Sprintf("New answer on question \"%s\" by %s", soAnswerUpdate.Title, soAnswerUpdate.Owner)
}

func (soAnswerUpdate *StackoverflowAnswerUpdate) Preview() string {
	return truncateText(soAnswerUpdate.Body, soAnswerUpdate.MaxPreviewLen)
}

type StackoverflowCommentUpdate struct {
	Title     string
	Owner     string
	CreatedAt time.Time
	Body      string

	MaxPreviewLen int
}

func (soCommentUpdate *StackoverflowCommentUpdate) UpdatedAt() time.Time {
	return soCommentUpdate.CreatedAt
}

func (soCommentUpdate *StackoverflowCommentUpdate) Description() string {
	return fmt.Sprintf("New comment on question \"%s\" by %s", soCommentUpdate.Title, soCommentUpdate.Owner)
}

func (soCommentUpdate *StackoverflowCommentUpdate) Preview() string {
	return truncateText(soCommentUpdate.Body, soCommentUpdate.MaxPreviewLen)
}

func truncateText(s string, maxLen int) string {
	runes := []rune(s)

	if len(runes) <= maxLen {
		return s
	}

	return string(runes[:maxLen-3]) + "..."
}
